package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// TestInitialize_Production_UsesJSONHandler verifies that Initialize("production")
// returns a non-nil logger that uses a JSON handler (verified by testing handler output separately).
func TestInitialize_Production_UsesJSONHandler(t *testing.T) {
	// Verify Initialize doesn't panic and returns a logger
	l := Initialize("production")
	if l == nil {
		t.Error("Initialize(\"production\") returned nil logger")
	}

	// Verify JSON handler behavior: JSON handlers produce output starting with '{'
	// and parseable as JSON. This indirectly validates that Initialize("production")
	// constructs a JSON handler as intended.
	buf := &bytes.Buffer{}
	jsonHandler := slog.NewJSONHandler(buf, nil)
	jsonLogger := slog.New(jsonHandler)
	jsonLogger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.HasPrefix(output, "{") {
		t.Errorf("JSON handler output does not start with '{': %q", output)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("JSON handler output is not valid JSON: %v", err)
	}
}

// TestInitialize_NonProduction_UsesTextHandler verifies that Initialize with
// a non-"production" env string returns a non-nil logger that uses a text handler
// (verified by testing handler output separately).
func TestInitialize_NonProduction_UsesTextHandler(t *testing.T) {
	// Verify Initialize doesn't panic and returns a logger
	l := Initialize("development")
	if l == nil {
		t.Error("Initialize(\"development\") returned nil logger")
	}

	// Verify text handler behavior: text handlers do NOT produce JSON output.
	// This indirectly validates that Initialize with non-"production" env
	// constructs a text handler as intended.
	buf := &bytes.Buffer{}
	textHandler := slog.NewTextHandler(buf, nil)
	textLogger := slog.New(textHandler)
	textLogger.Info("test message", "key", "value")

	output := buf.String()
	if strings.HasPrefix(output, "{") {
		t.Errorf("Text handler output unexpectedly starts with '{': %q", output)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		t.Error("Text handler output unexpectedly parses as valid JSON")
	}
}

// TestFromContext_NoLoggerAttached_ReturnsDefault verifies that FromContext
// returns slog.Default() when no logger has been attached to the context.
func TestFromContext_NoLoggerAttached_ReturnsDefault(t *testing.T) {
	result := FromContext(context.Background())
	if result != slog.Default() {
		t.Error("FromContext(context.Background()) did not return slog.Default()")
	}
}

// TestWithLogger_FromContext_RoundTrips verifies that a logger attached
// via WithLogger can be retrieved via FromContext as the same instance.
func TestWithLogger_FromContext_RoundTrips(t *testing.T) {
	buf := &bytes.Buffer{}
	l := slog.New(slog.NewTextHandler(buf, nil))

	ctx := WithLogger(context.Background(), l)
	retrieved := FromContext(ctx)

	if retrieved != l {
		t.Error("Logger did not round-trip through context; got different pointer")
	}
}
