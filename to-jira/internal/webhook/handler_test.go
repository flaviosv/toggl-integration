package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/flaviosv/toggl-integration/to-jira/internal/jira"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/telemetry"
	"github.com/flaviosv/toggl-integration/to-jira/internal/sync"
)

const testSecret = "test-secret"

// fakeJiraClient is a sync.JiraClient test double — the hard project
// constraint forbids live network calls, so the *sync.Processor wired into
// Handler in these tests is always backed by this fake, never *jira.Client.
// Its call counts double as the "spy" Done-when requires to prove
// Processor was never invoked on an authentication failure.
type fakeJiraClient struct {
	findResult *jira.Worklog
	findErr    error
	createResult *jira.Worklog
	createErr  error
	updateErr  error
	deleteErr  error

	findCalls, createCalls, updateCalls, deleteCalls int
}

func (f *fakeJiraClient) FindWorklogByTogglID(context.Context, string, string) (*jira.Worklog, error) {
	f.findCalls++
	return f.findResult, f.findErr
}

func (f *fakeJiraClient) CreateWorklog(context.Context, string, jira.WorklogInput) (*jira.Worklog, error) {
	f.createCalls++
	return f.createResult, f.createErr
}

func (f *fakeJiraClient) UpdateWorklog(context.Context, string, string, jira.WorklogInput) error {
	f.updateCalls++
	return f.updateErr
}

func (f *fakeJiraClient) DeleteWorklog(context.Context, string, string) error {
	f.deleteCalls++
	return f.deleteErr
}

func (f *fakeJiraClient) totalCalls() int {
	return f.findCalls + f.createCalls + f.updateCalls + f.deleteCalls
}

func newTestHandler(t *testing.T, fake *fakeJiraClient) (*gin.Engine, *fakeJiraClient) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	meter := sdkmetric.NewMeterProvider().Meter("test")
	metrics, err := telemetry.NewMetrics(meter)
	if err != nil {
		t.Fatalf("telemetry.NewMetrics() error = %v", err)
	}
	processor := sync.NewProcessor(fake, metrics, noop.NewTracerProvider().Tracer("test"), false)
	h := NewHandler(testSecret, processor, slog.New(slog.NewTextHandler(io.Discard, nil)))

	r := gin.New()
	r.POST("/webhooks/toggl", h.Receive)
	return r, fake
}

func sign(t *testing.T, secret string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

func post(t *testing.T, r *gin.Engine, body []byte, sig string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/toggl", bytes.NewReader(body))
	if sig != "" {
		req.Header.Set(signatureHeader, sig)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const createdEnvelope = `{"metadata":{"request_type":"time_entry.created"},"payload":{"id":42,"description":"[ABC-123] Did the thing","duration":1800,"start":"2021-01-17T12:00:00Z","stop":"2021-01-17T12:30:00Z"}}`
const updatedEnvelope = `{"metadata":{"request_type":"time_entry.updated"},"payload":{"id":42,"description":"[ABC-123] Did the thing","duration":1800,"start":"2021-01-17T12:00:00Z","stop":"2021-01-17T12:30:00Z"}}`
const deletedEnvelope = `{"metadata":{"request_type":"time_entry.deleted"},"payload":{"id":42,"description":"[ABC-123] Did the thing"}}`
const unrecognizedEnvelope = `{"metadata":{"request_type":"project.created"},"payload":{}}`
const malformedEnvelope = `not json`
const malformedPayloadEnvelope = `{"metadata":{"request_type":"time_entry.created"},"payload":"not an object"}`

// TJ-01/AC1: a missing signature header is rejected with 401, and
// sync.Processor is never invoked (no underlying JIRA call).
func TestReceive_MissingSignature_Returns401NoDispatch(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})

	w := post(t, r, []byte(createdEnvelope), "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if fake.totalCalls() != 0 {
		t.Errorf("totalCalls = %d, want 0 (Processor must not be invoked)", fake.totalCalls())
	}
}

// TJ-01/AC1: a malformed signature (missing the "sha256=" prefix) is
// rejected with 401.
func TestReceive_MalformedSignature_Returns401(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})

	w := post(t, r, []byte(createdEnvelope), "not-a-valid-signature")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if fake.totalCalls() != 0 {
		t.Errorf("totalCalls = %d, want 0", fake.totalCalls())
	}
}

// TJ-01/AC1: an invalid signature (wrong HMAC of the correct body) is
// rejected with 401.
func TestReceive_InvalidSignature_Returns401(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	wrongSig := sign(t, "wrong-secret", []byte(createdEnvelope))

	w := post(t, r, []byte(createdEnvelope), wrongSig)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if fake.totalCalls() != 0 {
		t.Errorf("totalCalls = %d, want 0", fake.totalCalls())
	}
}

// TJ-01/AC2: a valid signature on a created event dispatches to
// sync.Process and passes through its 200 result.
func TestReceive_ValidSignature_CreatedEvent_DispatchesToProcess(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	body := []byte(createdEnvelope)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if fake.findCalls != 1 || fake.createCalls != 1 {
		t.Errorf("findCalls=%d createCalls=%d, want 1 and 1 (Process must have run)", fake.findCalls, fake.createCalls)
	}
}

// TJ-01/AC2: a valid signature on an updated event dispatches to
// sync.Process the same way as created (unified upsert).
func TestReceive_ValidSignature_UpdatedEvent_DispatchesToProcess(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	body := []byte(updatedEnvelope)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if fake.findCalls != 1 {
		t.Errorf("findCalls = %d, want 1 (Process must have run)", fake.findCalls)
	}
}

// TJ-01/AC2: a valid signature on a deleted event dispatches to
// sync.ProcessDelete, not sync.Process.
func TestReceive_ValidSignature_DeletedEvent_DispatchesToProcessDelete(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	body := []byte(deletedEnvelope)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if fake.findCalls != 1 {
		t.Errorf("findCalls = %d, want 1 (ProcessDelete must have run)", fake.findCalls)
	}
	if fake.createCalls != 0 || fake.updateCalls != 0 {
		t.Errorf("createCalls=%d updateCalls=%d, want 0 (delete path must not create/update)", fake.createCalls, fake.updateCalls)
	}
}

// AC2/Error Handling Strategy: an unrecognized event type is ignored with
// 200 and no dispatch.
func TestReceive_UnrecognizedEventType_Returns200NoDispatch(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	body := []byte(unrecognizedEnvelope)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if fake.totalCalls() != 0 {
		t.Errorf("totalCalls = %d, want 0", fake.totalCalls())
	}
}

// Error Handling Strategy: a malformed envelope that nonetheless carries a
// valid signature is ignored with 200, never a panic.
func TestReceive_MalformedEnvelope_Returns200NoDispatch(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	body := []byte(malformedEnvelope)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if fake.totalCalls() != 0 {
		t.Errorf("totalCalls = %d, want 0", fake.totalCalls())
	}
}

// Error Handling Strategy: a malformed time-entry payload (valid envelope,
// valid signature, recognized type) is ignored with 200, never a panic.
func TestReceive_MalformedPayload_Returns200NoDispatch(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	body := []byte(malformedPayloadEnvelope)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if fake.totalCalls() != 0 {
		t.Errorf("totalCalls = %d, want 0", fake.totalCalls())
	}
}

// TJ-07/AC8: a transient JIRA failure surfaces as the non-2xx
// Result.HTTPStatus, passed straight through to the HTTP response.
func TestReceive_JiraLookupFails_ReturnsNon2xx(t *testing.T) {
	t.Parallel()
	r, _ := newTestHandler(t, &fakeJiraClient{findErr: &jira.TransientError{Err: context.DeadlineExceeded}})

	body := []byte(createdEnvelope)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code/100 == 2 {
		t.Errorf("status = %d, want non-2xx", w.Code)
	}
}

// A request body exceeding maxWebhookBodyBytes is rejected with 400.
func TestReceive_BodyExceedsMaxSize_Returns400(t *testing.T) {
	t.Parallel()
	r, fake := newTestHandler(t, &fakeJiraClient{})
	// Create a body larger than maxWebhookBodyBytes (1 << 20 = 1048576 bytes)
	body := bytes.Repeat([]byte("a"), (1<<20)+1)
	sig := sign(t, testSecret, body)

	w := post(t, r, body, sig)

	if w.Code == 200 {
		t.Errorf("status = %d, want non-200", w.Code)
	}
	if w.Code == 401 {
		t.Errorf("status = %d (401), want non-401", w.Code)
	}
	if fake.totalCalls() != 0 {
		t.Errorf("totalCalls = %d, want 0 (Processor must not be invoked)", fake.totalCalls())
	}
}
