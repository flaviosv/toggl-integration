package sync

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/flaviosv/toggl-integration/to-jira/internal/jira"
	"github.com/flaviosv/toggl-integration/to-jira/internal/toggl"
)

func completeEvent() toggl.Event {
	return toggl.Event{
		TogglID:     "42",
		Description: "[ABC-123] Did the thing",
		Duration:    30 * time.Minute,
		StartedAt:   time.Date(2021, 1, 17, 12, 0, 0, 0, time.UTC),
		HasStopped:  true,
	}
}

// TJ-02/AC2: an invalid description skips syncing, logs a validation error,
// increments validation_errors_total, and returns 200 with no JIRA call.
func TestProcess_InvalidDescription_SkipsAndCountsValidationError(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	e := completeEvent()
	e.Description = "not tagged"

	got := p.Process(context.Background(), e)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeSkippedInvalid}) {
		t.Errorf("Process() = %+v, want {200 skipped_invalid}", got)
	}
	if fake.findResult != nil || fake.createCalls != 0 || fake.updateCalls != 0 {
		t.Errorf("expected no JIRA calls, got createCalls=%d updateCalls=%d", fake.createCalls, fake.updateCalls)
	}
	if got := counterValue(t, reader, "validation_errors_total"); got != 1 {
		t.Errorf("validation_errors_total = %d, want 1", got)
	}
}

// TJ-03/AC3: a still-running entry (HasStopped=false) skips syncing with no
// JIRA call and no error, returning 200.
func TestProcess_RunningEntry_SkipsWithNoJiraCall(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	e := completeEvent()
	e.HasStopped = false

	got := p.Process(context.Background(), e)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeSkippedRunning}) {
		t.Errorf("Process() = %+v, want {200 skipped_running}", got)
	}
	if fake.createCalls != 0 || fake.updateCalls != 0 {
		t.Errorf("expected no JIRA write calls, got createCalls=%d updateCalls=%d", fake.createCalls, fake.updateCalls)
	}
}

// TJ-04/TJ-05/AC5: no matching worklog found creates one with the correct
// timeSpentSeconds, started, and comment.
func TestProcess_NoExistingWorklog_Creates(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: nil, createResult: &jira.Worklog{ID: "999"}}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	e := completeEvent()

	got := p.Process(context.Background(), e)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeCreated}) {
		t.Errorf("Process() = %+v, want {200 created}", got)
	}
	if fake.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", fake.createCalls)
	}
	if fake.lastIssueKey != "ABC-123" {
		t.Errorf("issueKey = %q, want ABC-123", fake.lastIssueKey)
	}
	if fake.lastInput.TimeSpentSeconds != 1800 {
		t.Errorf("TimeSpentSeconds = %d, want 1800", fake.lastInput.TimeSpentSeconds)
	}
	if !fake.lastInput.Started.Equal(e.StartedAt) {
		t.Errorf("Started = %v, want %v", fake.lastInput.Started, e.StartedAt)
	}
	if id, ok := jira.ExtractTogglID(fake.lastInput.Comment); !ok || id != "42" {
		t.Errorf("Comment TogglID = %q (ok=%v), want 42", id, ok)
	}
	if got := counterValue(t, reader, "worklogs_created_total"); got != 1 {
		t.Errorf("worklogs_created_total = %d, want 1", got)
	}
}

// TJ-04/TJ-06/AC6: a matching worklog updates in place instead of creating a
// duplicate.
func TestProcess_ExistingWorklog_Updates(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "555"}}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	e := completeEvent()

	got := p.Process(context.Background(), e)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeUpdated}) {
		t.Errorf("Process() = %+v, want {200 updated}", got)
	}
	if fake.updateCalls != 1 {
		t.Fatalf("updateCalls = %d, want 1", fake.updateCalls)
	}
	if fake.createCalls != 0 {
		t.Errorf("createCalls = %d, want 0 (must not duplicate)", fake.createCalls)
	}
	if fake.lastWorklogID != "555" {
		t.Errorf("worklogID = %q, want 555", fake.lastWorklogID)
	}
	if got := counterValue(t, reader, "worklogs_updated_total"); got != 1 {
		t.Errorf("worklogs_updated_total = %d, want 1", got)
	}
}

// TJ-07/AC8: a lookup failure is a transient error — non-2xx so Toggl
// retries, jira_api_errors_total incremented.
func TestProcess_LookupFails_ReturnsTransientError(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{findErr: &jira.TransientError{Err: context.DeadlineExceeded}}
	p := NewProcessor(fake, metrics, noopTracer(), false)

	got := p.Process(context.Background(), completeEvent())

	if got.Outcome != OutcomeTransientError {
		t.Errorf("Outcome = %q, want transient_error", got.Outcome)
	}
	if got.HTTPStatus/100 == 2 {
		t.Errorf("HTTPStatus = %d, want non-2xx", got.HTTPStatus)
	}
	if got := counterValue(t, reader, "jira_api_errors_total"); got != 1 {
		t.Errorf("jira_api_errors_total = %d, want 1", got)
	}
}

// TJ-07/AC8: a create failure (permanent, e.g. 404) also maps to
// transient_error/non-2xx per design.md's Error Handling Strategy table.
func TestProcess_CreateFails_ReturnsTransientError(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{createErr: &jira.PermanentError{StatusCode: 404, Err: context.DeadlineExceeded}}
	p := NewProcessor(fake, metrics, noopTracer(), false)

	got := p.Process(context.Background(), completeEvent())

	if got.Outcome != OutcomeTransientError {
		t.Errorf("Outcome = %q, want transient_error", got.Outcome)
	}
	if got.HTTPStatus/100 == 2 {
		t.Errorf("HTTPStatus = %d, want non-2xx", got.HTTPStatus)
	}
	if got := counterValue(t, reader, "jira_api_errors_total"); got != 1 {
		t.Errorf("jira_api_errors_total = %d, want 1", got)
	}
}

// TJ-07/AC8: an update failure also maps to transient_error/non-2xx.
func TestProcess_UpdateFails_ReturnsTransientError(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "555"}, updateErr: &jira.TransientError{Err: context.DeadlineExceeded}}
	p := NewProcessor(fake, metrics, noopTracer(), false)

	got := p.Process(context.Background(), completeEvent())

	if got.Outcome != OutcomeTransientError {
		t.Errorf("Outcome = %q, want transient_error", got.Outcome)
	}
	if got.HTTPStatus/100 == 2 {
		t.Errorf("HTTPStatus = %d, want non-2xx", got.HTTPStatus)
	}
}

// TJ-09/AC9: dry-run mode reaches the lookup (decision point) but never
// calls create/update — the write methods are never invoked.
func TestProcess_DryRun_Create_NeverCallsWrite(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: nil}
	p := NewProcessor(fake, metrics, noopTracer(), true)

	got := p.Process(context.Background(), completeEvent())

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeCreated}) {
		t.Errorf("Process() = %+v, want {200 created}", got)
	}
	if fake.createCalls != 0 {
		t.Errorf("createCalls = %d, want 0 in dry-run", fake.createCalls)
	}
	if got := counterValue(t, reader, "worklogs_created_total"); got != 0 {
		t.Errorf("worklogs_created_total = %d, want 0 in dry-run (no real write occurred)", got)
	}
}

// Edge case (spec.md): a duplicate delivery for a TogglID already synced
// updates in place across repeated calls, never creating a duplicate.
func TestProcess_RepeatedDelivery_NeverDuplicates(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "555"}}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	e := completeEvent()

	p.Process(context.Background(), e)
	p.Process(context.Background(), e)

	if fake.createCalls != 0 {
		t.Errorf("createCalls = %d, want 0 across repeated delivery", fake.createCalls)
	}
	if fake.updateCalls != 2 {
		t.Errorf("updateCalls = %d, want 2 (idempotent update, not duplicate create)", fake.updateCalls)
	}
}

// AC7: on success, Process emits a trace span tagged with the TogglID.
func TestProcess_Success_EmitsSpanTaggedWithTogglID(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	tracer, exporter := newRecordingTracer(t)
	fake := &fakeJiraClient{findResult: nil, createResult: &jira.Worklog{ID: "999"}}
	p := NewProcessor(fake, metrics, tracer, false)

	p.Process(context.Background(), completeEvent())

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Name != "sync.Process" {
		t.Errorf("span name = %q, want sync.Process", spans[0].Name)
	}
	want := attribute.String("toggl.id", "42")
	found := false
	for _, attr := range spans[0].Attributes {
		if attr == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("span attributes = %+v, want to include %v", spans[0].Attributes, want)
	}
}

// TJ-09/AC9: dry-run mode with an existing worklog reaches the lookup but
// never calls update.
func TestProcess_DryRun_Update_NeverCallsWrite(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "555"}}
	p := NewProcessor(fake, metrics, noopTracer(), true)

	got := p.Process(context.Background(), completeEvent())

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeUpdated}) {
		t.Errorf("Process() = %+v, want {200 updated}", got)
	}
	if fake.updateCalls != 0 {
		t.Errorf("updateCalls = %d, want 0 in dry-run", fake.updateCalls)
	}
}
