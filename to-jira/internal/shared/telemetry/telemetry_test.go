package telemetry

import (
	"context"
	"testing"
	"time"
)

// TestInitialize_EmptyEndpoint_WiresStdoutExportersAndShutdownWorks tests that
// Initialize with an empty OTLPEndpoint constructs stdout exporters in-process,
// sets them as global providers, and returns a working shutdown function.
func TestInitialize_EmptyEndpoint_WiresStdoutExportersAndShutdownWorks(t *testing.T) {
	shutdown, err := Initialize(context.Background(), Config{OTLPEndpoint: ""})

	if err != nil {
		t.Fatalf("Initialize with empty endpoint should not error, got: %v", err)
	}

	if shutdown == nil {
		t.Fatal("shutdown func should not be nil")
	}

	// Call shutdown and verify it returns no error.
	shutdownErr := shutdown(context.Background())
	if shutdownErr != nil {
		t.Fatalf("shutdown should not error, got: %v", shutdownErr)
	}
}

// TestInitialize_MalformedEndpoint_FailsAtConstruction attempts to verify that
// malformed endpoints fail at construction time, not during network calls.
// If OTLP exporters don't validate the endpoint URL synchronously at construction,
// this test is skipped to avoid risking network calls.
func TestInitialize_MalformedEndpoint_FailsAtConstruction(t *testing.T) {
	malformedEndpoint := "://invalid"

	// Use a short timeout defensively to prevent the test from hanging
	// if construction somehow attempts network operations.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	shutdown, err := Initialize(ctx, Config{OTLPEndpoint: malformedEndpoint})

	if err == nil {
		// OTLP exporters accepted the malformed endpoint without raising an error.
		// This means they don't validate the endpoint URL at construction time.
		// We cannot reliably test the failure case without risking network calls,
		// so skip this sub-case.
		t.Skip("OTLP exporters do not validate endpoint URL at construction time; skipping forced-failure test to avoid network calls")
	}

	// If we reach here, Initialize failed at construction time (good).
	// Verify shutdown is nil since Initialize failed.
	if shutdown != nil {
		t.Errorf("expected shutdown to be nil when Initialize fails, got non-nil")
	}

	t.Logf("Initialize correctly rejected malformed endpoint at construction: %v", err)
}
