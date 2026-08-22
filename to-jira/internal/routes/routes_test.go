package routes

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
	"github.com/flaviosv/toggl-integration/to-jira/internal/webhook"
)

const testSecret = "test-secret"

// fakeJiraClient is a sync.JiraClient test double.
type fakeJiraClient struct {
	findResult   *jira.Worklog
	createResult *jira.Worklog
	findCalls    int
}

func (f *fakeJiraClient) FindWorklogByTogglID(context.Context, string, string) (*jira.Worklog, error) {
	f.findCalls++
	return f.findResult, nil
}

func (f *fakeJiraClient) CreateWorklog(context.Context, string, jira.WorklogInput) (*jira.Worklog, error) {
	return f.createResult, nil
}

func (f *fakeJiraClient) UpdateWorklog(context.Context, string, string, jira.WorklogInput) error {
	return nil
}

func (f *fakeJiraClient) DeleteWorklog(context.Context, string, string) error {
	return nil
}

func signWithSecret(t *testing.T, secret string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Routes must register the endpoint on the given group, not on app directly,
// so that middleware attached to group actually runs on the route.
func TestRoutes_RegistersOnGroupWithMiddlewareIntact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create the engine and a router group.
	engine := gin.New()
	group := engine.Group("/")

	// Attach marker middleware to the group to verify it runs.
	var middlewareRan bool
	group.Use(func(c *gin.Context) {
		middlewareRan = true
		c.Next()
	})

	// Construct a minimal webhook.Handler.
	fake := &fakeJiraClient{}
	meter := sdkmetric.NewMeterProvider().Meter("test")
	metrics, err := telemetry.NewMetrics(meter)
	if err != nil {
		t.Fatalf("telemetry.NewMetrics() error = %v", err)
	}
	processor := sync.NewProcessor(fake, metrics, noop.NewTracerProvider().Tracer("test"), false)
	handler := webhook.NewHandler(testSecret, processor, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Register the route via Routes.
	Routes(group, handler)

	// Make a validly-signed request.
	body := []byte(`{"metadata":{"request_type":"time_entry.created"},"payload":{"id":42,"description":"[ABC-123] Did the thing","duration":1800,"start":"2021-01-17T12:00:00Z","stop":"2021-01-17T12:30:00Z"}}`)
	sig := signWithSecret(t, testSecret, body)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/toggl", bytes.NewReader(body))
	req.Header.Set("X-Webhook-Signature-256", sig)
	w := httptest.NewRecorder()

	// Serve the request.
	engine.ServeHTTP(w, req)

	// Assert that the handler ran (200 response).
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	// Assert that the middleware ran.
	if !middlewareRan {
		t.Error("middleware did not run, but it should have (registered on group)")
	}
}
