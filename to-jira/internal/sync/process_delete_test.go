package sync

import (
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/flaviosv/toggl-integration/to-jira/internal/jira"
	"github.com/flaviosv/toggl-integration/to-jira/internal/toggl"
)

// deleteEnvelope builds a WebhookEnvelope carrying a delete-event payload.
// descPtr models the two hypothesized delete-payload shapes from design.md's
// Risks: shape A (description present, other fields nil) and shape B
// (description nil).
func deleteEnvelope(t *testing.T, id int64, descPtr *string) toggl.WebhookEnvelope {
	t.Helper()
	payload := toggl.TimeEntryPayload{ID: id, Description: descPtr}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}
	return toggl.WebhookEnvelope{
		Metadata: toggl.EventMetadata{RequestType: "time_entry.deleted"},
		Payload:  raw,
	}
}

func descPtr(s string) *string { return &s }

// TJ-10/AC1-2 (hypothesized shape A): a delete payload with a derivable
// description deletes the matching worklog on the derived issue.
func TestProcessDelete_DerivablePayload_DeletesMatchingWorklog(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "777"}}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	env := deleteEnvelope(t, 42, descPtr("[ABC-123] Did the thing"))

	got := p.ProcessDelete(context.Background(), env)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeDeleted}) {
		t.Errorf("ProcessDelete() = %+v, want {200 deleted}", got)
	}
	if fake.lastFindIssueKey != "ABC-123" || fake.lastFindTogglID != "42" {
		t.Errorf("Find called with (%q, %q), want (ABC-123, 42)", fake.lastFindIssueKey, fake.lastFindTogglID)
	}
	if fake.deleteCalls != 1 {
		t.Fatalf("deleteCalls = %d, want 1", fake.deleteCalls)
	}
	if fake.lastIssueKey != "ABC-123" || fake.lastWorklogID != "777" {
		t.Errorf("Delete called with (%q, %q), want (ABC-123, 777)", fake.lastIssueKey, fake.lastWorklogID)
	}
	if got := counterValue(t, reader, "worklogs_deleted_total"); got != 1 {
		t.Errorf("worklogs_deleted_total = %d, want 1", got)
	}
}

// TJ-11/AC3: no matching worklog on the derived issue is a no-op success,
// not an error.
func TestProcessDelete_NoMatchingWorklog_Noop(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: nil}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	env := deleteEnvelope(t, 42, descPtr("[ABC-123] Did the thing"))

	got := p.ProcessDelete(context.Background(), env)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeNoop}) {
		t.Errorf("ProcessDelete() = %+v, want {200 noop}", got)
	}
	if fake.deleteCalls != 0 {
		t.Errorf("deleteCalls = %d, want 0", fake.deleteCalls)
	}
}

// TJ-12/AC4 (hypothesized shape B): a delete payload with a nil description
// cannot derive an issue key — logged as unsupported-delete, 200,
// validation_errors_total incremented.
func TestProcessDelete_NonDerivablePayload_UnsupportedDelete(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	env := deleteEnvelope(t, 42, nil)

	got := p.ProcessDelete(context.Background(), env)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeUnsupportedDelete}) {
		t.Errorf("ProcessDelete() = %+v, want {200 unsupported_delete}", got)
	}
	if fake.findCalls != 0 || fake.deleteCalls != 0 {
		t.Errorf("expected no JIRA calls, got findCalls=%d deleteCalls=%d", fake.findCalls, fake.deleteCalls)
	}
	if got := counterValue(t, reader, "validation_errors_total"); got != 1 {
		t.Errorf("validation_errors_total = %d, want 1", got)
	}
}

// TJ-12/AC4: a description present but not matching the required tagged
// format is equally non-derivable.
func TestProcessDelete_UntaggedDescription_UnsupportedDelete(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	env := deleteEnvelope(t, 42, descPtr("no tag here"))

	got := p.ProcessDelete(context.Background(), env)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeUnsupportedDelete}) {
		t.Errorf("ProcessDelete() = %+v, want {200 unsupported_delete}", got)
	}
}

// TJ-12: a structurally malformed payload (not even valid JSON for
// TimeEntryPayload) is equally non-derivable — never a panic.
func TestProcessDelete_MalformedPayload_UnsupportedDelete(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	env := toggl.WebhookEnvelope{
		Metadata: toggl.EventMetadata{RequestType: "time_entry.deleted"},
		Payload:  []byte(`"not an object"`),
	}

	got := p.ProcessDelete(context.Background(), env)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeUnsupportedDelete}) {
		t.Errorf("ProcessDelete() = %+v, want {200 unsupported_delete}", got)
	}
}

// TJ-13/AC5: a transient JIRA lookup failure returns non-2xx for Toggl
// retry.
func TestProcessDelete_LookupFails_ReturnsTransientError(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{findErr: &jira.TransientError{Err: context.DeadlineExceeded}}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	env := deleteEnvelope(t, 42, descPtr("[ABC-123] Did the thing"))

	got := p.ProcessDelete(context.Background(), env)

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

// TJ-13: a JIRA delete-call failure returns non-2xx for Toggl retry.
func TestProcessDelete_DeleteFails_ReturnsTransientError(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "777"}, deleteErr: &jira.TransientError{Err: context.DeadlineExceeded}}
	p := NewProcessor(fake, metrics, noopTracer(), false)
	env := deleteEnvelope(t, 42, descPtr("[ABC-123] Did the thing"))

	got := p.ProcessDelete(context.Background(), env)

	if got.Outcome != OutcomeTransientError {
		t.Errorf("Outcome = %q, want transient_error", got.Outcome)
	}
	if got.HTTPStatus/100 == 2 {
		t.Errorf("HTTPStatus = %d, want non-2xx", got.HTTPStatus)
	}
}

// TJ-13/AC6: dry-run mode reaches the lookup but never calls delete.
func TestProcessDelete_DryRun_NeverCallsDelete(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "777"}}
	p := NewProcessor(fake, metrics, noopTracer(), true)
	env := deleteEnvelope(t, 42, descPtr("[ABC-123] Did the thing"))

	got := p.ProcessDelete(context.Background(), env)

	if got != (Result{HTTPStatus: 200, Outcome: OutcomeDeleted}) {
		t.Errorf("ProcessDelete() = %+v, want {200 deleted}", got)
	}
	if fake.deleteCalls != 0 {
		t.Errorf("deleteCalls = %d, want 0 in dry-run", fake.deleteCalls)
	}
	if got := counterValue(t, reader, "worklogs_deleted_total"); got != 0 {
		t.Errorf("worklogs_deleted_total = %d, want 0 in dry-run", got)
	}
}

// AC7: on success, ProcessDelete emits a trace span tagged with the TogglID.
func TestProcessDelete_Success_EmitsSpanTaggedWithTogglID(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	tracer, exporter := newRecordingTracer(t)
	fake := &fakeJiraClient{findResult: &jira.Worklog{ID: "777"}}
	p := NewProcessor(fake, metrics, tracer, false)
	env := deleteEnvelope(t, 42, descPtr("[ABC-123] Did the thing"))

	p.ProcessDelete(context.Background(), env)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Name != "sync.ProcessDelete" {
		t.Errorf("span name = %q, want sync.ProcessDelete", spans[0].Name)
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
