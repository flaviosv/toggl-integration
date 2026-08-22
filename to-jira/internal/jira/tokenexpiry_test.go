package jira

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func testLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

// WarnIfTokenExpiringSoon must log a warning when expiry is within 14 days.
func TestWarnIfTokenExpiringSoon_WarnsWithinWindow(t *testing.T) {
	logger, buf := testLogger()
	expiry := time.Now().Add(10 * 24 * time.Hour)

	c := NewClient("https://example.atlassian.net", "user@example.com", "token", nil)
	c.WarnIfTokenExpiringSoon(&expiry, logger)

	if !strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("log output = %q, want a level=WARN line", buf.String())
	}
}

// WarnIfTokenExpiringSoon must log an informational (non-warning) note when
// configuredExpiry is nil.
func TestWarnIfTokenExpiringSoon_NilExpiry_LogsInfo(t *testing.T) {
	logger, buf := testLogger()

	c := NewClient("https://example.atlassian.net", "user@example.com", "token", nil)
	c.WarnIfTokenExpiringSoon(nil, logger)

	out := buf.String()
	if !strings.Contains(out, "level=INFO") {
		t.Errorf("log output = %q, want a level=INFO line", out)
	}
	if strings.Contains(out, "level=WARN") {
		t.Errorf("log output = %q, want no level=WARN line", out)
	}
}

// WarnIfTokenExpiringSoon must log nothing when expiry is comfortably far
// away.
func TestWarnIfTokenExpiringSoon_FarAway_NoLog(t *testing.T) {
	logger, buf := testLogger()
	expiry := time.Now().Add(60 * 24 * time.Hour)

	c := NewClient("https://example.atlassian.net", "user@example.com", "token", nil)
	c.WarnIfTokenExpiringSoon(&expiry, logger)

	if buf.Len() != 0 {
		t.Errorf("log output = %q, want no output", buf.String())
	}
}
