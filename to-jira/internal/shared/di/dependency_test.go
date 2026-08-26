package di

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/metric"

	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/config"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/telemetry"
)

func testConfig() *config.Config {
	return &config.Config{
		TogglWebhookSecret: "secret",
		Jira: config.JiraConfig{
			BaseURL:  "https://example.atlassian.net",
			Email:    "user@example.com",
			APIToken: "token",
		},
		Port: 8080,
	}
}

// BuildDependencies must wire every component with no nil fields on
// success.
func TestBuildDependencies_WiresAllComponentsWithNoNilFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	d, err := BuildDependencies(testConfig(), logger)

	if err != nil {
		t.Fatalf("BuildDependencies() error = %v, want nil", err)
	}
	if d.Processor == nil {
		t.Error("d.Processor = nil, want a wired *sync.Processor")
	}
	if d.Handlers.Webhook == nil {
		t.Error("d.Handlers.Webhook = nil, want a wired *webhook.Handler")
	}
	if d.metrics == nil {
		t.Error("d.metrics = nil, want wired telemetry.Metrics")
	}
	if d.tracer == nil {
		t.Error("d.tracer = nil, want a wired trace.Tracer")
	}
	if d.clients.jira == nil {
		t.Error("d.clients.jira = nil, want a wired *jira.Client")
	}
}

// A build-stage failure (telemetry wiring) surfaces as an error, not a
// panic.
func TestBuildDependencies_TelemetryFailure_ReturnsError(t *testing.T) {
	original := newMetrics
	newMetrics = func(metric.Meter) (*telemetry.Metrics, error) {
		return nil, errors.New("forced failure")
	}
	defer func() { newMetrics = original }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	d, err := BuildDependencies(testConfig(), logger)

	if err == nil {
		t.Fatal("BuildDependencies() error = nil, want an error")
	}
	if d != nil {
		t.Errorf("d = %+v, want nil on failure", d)
	}
}
