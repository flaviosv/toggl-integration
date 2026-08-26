package sync

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/flaviosv/toggl-integration/to-jira/internal/shared/telemetry"
)

// newTestMetrics builds real telemetry.Metrics counters backed by an
// in-memory manual reader, so tests can assert on actual recorded counts
// rather than a no-op stand-in that discards everything.
func newTestMetrics(t *testing.T) (*telemetry.Metrics, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m, err := telemetry.NewMetrics(mp.Meter("test"))
	if err != nil {
		t.Fatalf("telemetry.NewMetrics() error = %v", err)
	}
	return m, reader
}

// counterValue collects reader's current state and sums the data points for
// the named counter (0 if the counter was never incremented).
func counterValue(t *testing.T, reader *sdkmetric.ManualReader, name string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("reader.Collect() error = %v", err)
	}
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
		}
	}
	return total
}

// noopTracer returns a trace.Tracer that records nothing, for tests that
// don't assert on spans.
func noopTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("test")
}

// newRecordingTracer returns a trace.Tracer backed by an in-memory exporter,
// for the one test that asserts on the actual emitted span (AC7's
// span-tagging requirement) rather than discarding it.
func newRecordingTracer(t *testing.T) (trace.Tracer, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	return tp.Tracer("test"), exporter
}
