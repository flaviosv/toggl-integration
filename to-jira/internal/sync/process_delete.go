package sync

import (
	"context"
	"encoding/json"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/logger"
	"github.com/flaviosv/toggl-integration/to-jira/internal/toggl"
)

// ProcessDelete orchestrates TJ-10..TJ-13 for a deleted Toggl event: derive
// the JIRA issue key from the delete payload the same way as create/update
// (design.md's Risks flag the delete payload shape as unverified — both
// hypothesized shapes are handled), look up the matching worklog by TogglID
// marker, and delete it if found.
func (p *Processor) ProcessDelete(ctx context.Context, env toggl.WebhookEnvelope) Result {
	ctx, span := p.tracer.Start(ctx, "sync.ProcessDelete")
	defer span.End()

	var payload toggl.TimeEntryPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		logger.FromContext(ctx).Warn("sync: delete payload not derivable", "error", err)
		p.metrics.ValidationErrors.Add(ctx, 1)
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeUnsupportedDelete}
	}

	e := toggl.NormalizeEntry(payload)
	span.SetAttributes(attribute.String("toggl.id", e.TogglID))

	issueKey, _, ok := toggl.ParseDescription(e.Description)
	if !ok {
		logger.FromContext(ctx).Warn("sync: delete payload not derivable", "toggl_id", e.TogglID)
		p.metrics.ValidationErrors.Add(ctx, 1)
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeUnsupportedDelete}
	}

	existing, err := p.jira.FindWorklogByTogglID(ctx, issueKey, e.TogglID)
	if err != nil {
		return p.jiraError(ctx, "sync: jira lookup failed", e.TogglID, issueKey, err)
	}
	if existing == nil {
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeNoop}
	}

	if p.dryRun {
		logger.FromContext(ctx).Info("sync: dry-run, would delete worklog", "toggl_id", e.TogglID, "issue_key", issueKey, "worklog_id", existing.ID)
		return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeDeleted}
	}

	if err := p.jira.DeleteWorklog(ctx, issueKey, existing.ID); err != nil {
		return p.jiraError(ctx, "sync: jira delete failed", e.TogglID, issueKey, err)
	}
	p.metrics.WorklogsDeleted.Add(ctx, 1)
	return Result{HTTPStatus: http.StatusOK, Outcome: OutcomeDeleted}
}
