// Package sync orchestrates the Toggl-to-JIRA sync pipeline: parsing and
// validating Toggl events, looking up existing JIRA worklogs by TogglID
// marker, and creating, updating, or deleting them accordingly.
package sync

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/flaviosv/toggl-integration/to-jira/internal/jira"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/logger"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/telemetry"
	"github.com/flaviosv/toggl-integration/to-jira/internal/toggl"
)

// Outcome values a Result can carry — see design.md's Error Handling
// Strategy table.
const (
	OutcomeCreated           = "created"
	OutcomeUpdated           = "updated"
	OutcomeSkippedInvalid    = "skipped_invalid"
	OutcomeSkippedRunning    = "skipped_running"
	OutcomeDeleted           = "deleted"
	OutcomeNoop              = "noop"
	OutcomeUnsupportedDelete = "unsupported_delete"
	OutcomeTransientError    = "transient_error"
)

// Result is what webhook.Handler maps directly to its HTTP response.
type Result struct {
	HTTPStatus int
	Outcome    string
}

// JiraClient is the subset of *jira.Client the Processor depends on, defined
// as an interface so tests can fake it instead of making live network calls.
type JiraClient interface {
	FindWorklogByTogglID(ctx context.Context, issueKey, togglID string) (*jira.Worklog, error)
	CreateWorklog(ctx context.Context, issueKey string, in jira.WorklogInput) (*jira.Worklog, error)
	UpdateWorklog(ctx context.Context, issueKey, worklogID string, in jira.WorklogInput) error
	DeleteWorklog(ctx context.Context, issueKey, worklogID string) error
}

// Processor orchestrates TJ-02..TJ-13: validation, idempotent lookup, and
// create/update/delete dispatch against JIRA, with dry-run branching and
// OTel span/metric emission.
type Processor struct {
	jira    JiraClient
	metrics *telemetry.Metrics
	tracer  trace.Tracer
	dryRun  bool
}

// NewProcessor constructs a Processor.
func NewProcessor(client JiraClient, metrics *telemetry.Metrics, tracer trace.Tracer, dryRun bool) *Processor {
	return &Processor{jira: client, metrics: metrics, tracer: tracer, dryRun: dryRun}
}

// Process orchestrates TJ-02..TJ-09 for a created/updated Toggl event:
// description-format validation, running-entry skip, idempotent lookup by
// TogglID marker, and create-or-update against JIRA.
func (p *Processor) Process(ctx context.Context, e toggl.Event) Result {
	ctx, span := p.tracer.Start(ctx, "sync.Process")
	defer span.End()
	span.SetAttributes(attribute.String("toggl.id", e.TogglID))

	issueKey, text, ok := toggl.ParseDescription(e.Description)
	if !ok {
		logger.FromContext(ctx).Warn("sync: invalid description format", "toggl_id", e.TogglID, "description", e.Description)
		p.metrics.ValidationErrors.Add(ctx, 1)
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeSkippedInvalid}
	}

	if !e.HasStopped {
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeSkippedRunning}
	}

	existing, err := p.jira.FindWorklogByTogglID(ctx, issueKey, e.TogglID)
	if err != nil {
		logger.FromContext(ctx).Error("sync: jira lookup failed", "toggl_id", e.TogglID, "issue_key", issueKey, "error", err)
		p.metrics.JiraAPIErrors.Add(ctx, 1)
		return Result{HTTPStatus: http.StatusBadGateway, Outcome: OutcomeTransientError}
	}

	input := jira.WorklogInput{
		TimeSpentSeconds: int64(e.Duration.Seconds()),
		Started:          e.StartedAt,
		Comment:          jira.BuildComment(e.TogglID, text),
	}

	if existing != nil {
		return p.upsertUpdate(ctx, issueKey, existing.ID, input, e.TogglID)
	}
	return p.upsertCreate(ctx, issueKey, input, e.TogglID)
}

func (p *Processor) upsertUpdate(ctx context.Context, issueKey, worklogID string, input jira.WorklogInput, togglID string) Result {
	if p.dryRun {
		logger.FromContext(ctx).Info("sync: dry-run, would update worklog", "toggl_id", togglID, "issue_key", issueKey, "worklog_id", worklogID)
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeUpdated}
	}
	if err := p.jira.UpdateWorklog(ctx, issueKey, worklogID, input); err != nil {
		logger.FromContext(ctx).Error("sync: jira update failed", "toggl_id", togglID, "issue_key", issueKey, "error", err)
		p.metrics.JiraAPIErrors.Add(ctx, 1)
		return Result{HTTPStatus: http.StatusBadGateway, Outcome: OutcomeTransientError}
	}
	p.metrics.WorklogsUpdated.Add(ctx, 1)
	return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeUpdated}
}

func (p *Processor) upsertCreate(ctx context.Context, issueKey string, input jira.WorklogInput, togglID string) Result {
	if p.dryRun {
		logger.FromContext(ctx).Info("sync: dry-run, would create worklog", "toggl_id", togglID, "issue_key", issueKey)
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeCreated}
	}
	if _, err := p.jira.CreateWorklog(ctx, issueKey, input); err != nil {
		logger.FromContext(ctx).Error("sync: jira create failed", "toggl_id", togglID, "issue_key", issueKey, "error", err)
		p.metrics.JiraAPIErrors.Add(ctx, 1)
		return Result{HTTPStatus: http.StatusBadGateway, Outcome: OutcomeTransientError}
	}
	p.metrics.WorklogsCreated.Add(ctx, 1)
	return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeCreated}
}
