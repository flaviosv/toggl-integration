// Package di wires the service's dependencies via a staged builder,
// mirroring dinherim/applyr's internal/shared/di pattern minus
// buildDBs/buildRepositories (no persistence layer, per spec) — with
// buildClients (JIRA) and buildTelemetry stages added instead.
package di

import (
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/flaviosv/toggl-integration/to-jira/internal/jira"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/config"
	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/telemetry"
	"github.com/flaviosv/toggl-integration/to-jira/internal/sync"
	"github.com/flaviosv/toggl-integration/to-jira/internal/webhook"
)

const tracerName = "to-jira"

// newMetrics is swappable in tests to force a build failure — the same
// testability idiom as dinherim/applyr's `var dbNew = db.New`.
var newMetrics = telemetry.NewMetrics

type Dependency struct {
	metrics   *telemetry.Metrics
	tracer    trace.Tracer
	clients   clients
	Processor *sync.Processor
	Handlers  handlers
}

type clients struct {
	jira *jira.Client
}

type handlers struct {
	Webhook *webhook.Handler
}

// BuildDependencies wires telemetry metrics/tracer, the JIRA client, the
// sync orchestrator, and the webhook handler. baseLogger is shared with the
// caller (main.go) rather than built independently here, since
// config.Config carries no environment field for logger.Initialize to key
// off.
func BuildDependencies(cfg *config.Config, baseLogger *slog.Logger) (*Dependency, error) {
	d := &Dependency{}

	if err := d.buildTelemetry(); err != nil {
		return nil, fmt.Errorf("di: %w", err)
	}
	d.buildClients(cfg)
	d.buildProcessor(cfg)
	d.buildHandlers(cfg, baseLogger)

	return d, nil
}

func (d *Dependency) buildTelemetry() error {
	metrics, err := newMetrics(telemetry.Meter())
	if err != nil {
		return fmt.Errorf("build telemetry: %w", err)
	}
	d.metrics = metrics
	d.tracer = otel.Tracer(tracerName)
	return nil
}

func (d *Dependency) buildClients(cfg *config.Config) {
	d.clients.jira = jira.NewClient(cfg.Jira.BaseURL, cfg.Jira.Email, cfg.Jira.APIToken, nil)
}

func (d *Dependency) buildProcessor(cfg *config.Config) {
	d.Processor = sync.NewProcessor(d.clients.jira, d.metrics, d.tracer, cfg.DryRun)
}

func (d *Dependency) buildHandlers(cfg *config.Config, baseLogger *slog.Logger) {
	d.Handlers.Webhook = webhook.NewHandler(cfg.TogglWebhookSecret, d.Processor, baseLogger)
}
