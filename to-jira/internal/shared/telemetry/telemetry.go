package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const serviceName = "to-jira"

// Config carries only what telemetry bootstrap needs. OTLPEndpoint selects
// the exporter: empty uses a stdout/console exporter (AD-003 default),
// non-empty targets that OTLP HTTP endpoint.
type Config struct {
	OTLPEndpoint string
}

type Metrics struct {
	WorklogsCreated  metric.Int64Counter
	WorklogsUpdated  metric.Int64Counter
	WorklogsDeleted  metric.Int64Counter
	ValidationErrors metric.Int64Counter
	JiraAPIErrors    metric.Int64Counter
}

// Initialize bootstraps the OTel TracerProvider and MeterProvider, sets them
// as the global providers, and returns a shutdown func that flushes and
// closes both.
func Initialize(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	traceExporter, err := newTraceExporter(ctx, cfg.OTLPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("telemetry: trace exporter: %w", err)
	}

	metricExporter, err := newMetricExporter(ctx, cfg.OTLPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("telemetry: metric exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}

	return shutdown, nil
}

func newTraceExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	if endpoint == "" {
		return stdouttrace.New()
	}
	return otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
}

func newMetricExporter(ctx context.Context, endpoint string) (sdkmetric.Exporter, error) {
	if endpoint == "" {
		return stdoutmetric.New()
	}
	return otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(endpoint))
}

// NewMetrics creates the five counters this project emits against meter.
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	created, err1 := meter.Int64Counter("worklogs_created_total")
	updated, err2 := meter.Int64Counter("worklogs_updated_total")
	deleted, err3 := meter.Int64Counter("worklogs_deleted_total")
	validationErrors, err4 := meter.Int64Counter("validation_errors_total")
	jiraAPIErrors, err5 := meter.Int64Counter("jira_api_errors_total")

	if err := errors.Join(err1, err2, err3, err4, err5); err != nil {
		return nil, fmt.Errorf("telemetry: new metrics: %w", err)
	}

	return &Metrics{
		WorklogsCreated:  created,
		WorklogsUpdated:  updated,
		WorklogsDeleted:  deleted,
		ValidationErrors: validationErrors,
		JiraAPIErrors:    jiraAPIErrors,
	}, nil
}

// Meter returns the global meter for this service, for use with NewMetrics.
func Meter() metric.Meter {
	return otel.Meter(serviceName)
}
