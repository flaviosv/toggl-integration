package config

import (
	"strings"
	"testing"
	"time"
)

// configEnvKeys lists every env var Load() reads. Tests use this to force a
// fully known (empty) starting state before setting only what each case needs.
var configEnvKeys = []string{
	"TOGGL_WEBHOOK_SECRET",
	"JIRA_BASE_URL", "JIRA_EMAIL", "JIRA_API_TOKEN", "JIRA_API_TOKEN_EXPIRES_AT",
	"DRY_RUN", "OTEL_EXPORTER_OTLP_ENDPOINT", "PORT",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range configEnvKeys {
		t.Setenv(k, "")
	}
}

// setValidRequiredEnv sets only the required (no-default) env vars to valid
// values, leaving every optional var unset so default-application tests can
// layer on top.
func setValidRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TOGGL_WEBHOOK_SECRET", "webhook-secret")
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "user@example.com")
	t.Setenv("JIRA_API_TOKEN", "jira-token")
}

// TestLoad covers Load()'s full behavior: a fully valid config populates
// every field, a missing or malformed required field fails with an error
// naming that field, and an absent optional field falls back to its
// documented default.
func TestLoad(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr bool
		// errField is the field/env-var name expected in the error message
		// (wantErr cases only).
		errField string
		// check runs additional assertions against a successfully loaded Config.
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid full config",
			setup: func(t *testing.T) {
				clearConfigEnv(t)
				t.Setenv("TOGGL_WEBHOOK_SECRET", "webhook-secret")
				t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
				t.Setenv("JIRA_EMAIL", "user@example.com")
				t.Setenv("JIRA_API_TOKEN", "jira-token")
				t.Setenv("JIRA_API_TOKEN_EXPIRES_AT", "2026-12-31")
				t.Setenv("DRY_RUN", "true")
				t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
				t.Setenv("PORT", "9090")
			},
			check: func(t *testing.T, cfg *Config) {
				wantExpiry := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
				fields := []struct {
					name string
					got  any
					want any
				}{
					{"TogglWebhookSecret", cfg.TogglWebhookSecret, "webhook-secret"},
					{"Jira.BaseURL", cfg.Jira.BaseURL, "https://example.atlassian.net"},
					{"Jira.Email", cfg.Jira.Email, "user@example.com"},
					{"Jira.APIToken", cfg.Jira.APIToken, "jira-token"},
					{"DryRun", cfg.DryRun, true},
					{"OtelExporterOTLPEndpoint", cfg.OtelExporterOTLPEndpoint, "http://localhost:4317"},
					{"Port", cfg.Port, 9090},
				}
				for _, f := range fields {
					if f.got != f.want {
						t.Errorf("%s = %v, want %v", f.name, f.got, f.want)
					}
				}
				if cfg.Jira.APITokenExpires == nil || !cfg.Jira.APITokenExpires.Equal(wantExpiry) {
					t.Errorf("Jira.APITokenExpires = %v, want %v", cfg.Jira.APITokenExpires, wantExpiry)
				}
			},
		},

		// Missing required field: the field's env var is cleared while every
		// other required var stays valid; Load() must fail, naming that field.
		{name: "TOGGL_WEBHOOK_SECRET missing", setup: missingRequiredEnv("TOGGL_WEBHOOK_SECRET"), wantErr: true, errField: "TogglWebhookSecret"},
		{name: "JIRA_BASE_URL missing", setup: missingRequiredEnv("JIRA_BASE_URL"), wantErr: true, errField: "Jira.BaseURL"},
		{name: "JIRA_EMAIL missing", setup: missingRequiredEnv("JIRA_EMAIL"), wantErr: true, errField: "Jira.Email"},
		{name: "JIRA_API_TOKEN missing", setup: missingRequiredEnv("JIRA_API_TOKEN"), wantErr: true, errField: "Jira.APIToken"},

		// Malformed field: present but semantically invalid -> fails with a
		// specific error naming the field, not a panic.
		{name: "PORT non-numeric", setup: malformedEnv("PORT", "not-a-port"), wantErr: true, errField: "PORT"},
		{name: "DRY_RUN non-boolean", setup: malformedEnv("DRY_RUN", "maybe"), wantErr: true, errField: "DRY_RUN"},
		{name: "JIRA_API_TOKEN_EXPIRES_AT invalid date", setup: malformedEnv("JIRA_API_TOKEN_EXPIRES_AT", "not-a-date"), wantErr: true, errField: "JIRA_API_TOKEN_EXPIRES_AT"},

		// Optional field absent -> documented default applied.
		{name: "DryRun default", setup: optionalUnset, check: wantField(func(c *Config) any { return c.DryRun }, defaultDryRun)},
		{name: "Port default", setup: optionalUnset, check: wantField(func(c *Config) any { return c.Port }, defaultPort)},
		{name: "OtelExporterOTLPEndpoint default", setup: optionalUnset, check: wantField(func(c *Config) any { return c.OtelExporterOTLPEndpoint }, "")},
		{name: "Jira.APITokenExpires default", setup: optionalUnset, check: func(t *testing.T, cfg *Config) {
			if cfg.Jira.APITokenExpires != nil {
				t.Errorf("Jira.APITokenExpires = %v, want nil", cfg.Jira.APITokenExpires)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)

			cfg, err := Load()

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Load() expected error, got nil (cfg=%+v)", cfg)
				}
				if cfg != nil {
					t.Fatalf("Load() expected nil *Config on error, got %+v", cfg)
				}
				if !containsField(err.Error(), tc.errField) {
					t.Fatalf("Load() error %q does not name field %q", err.Error(), tc.errField)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() returned unexpected error: %v", err)
			}
			if cfg == nil {
				t.Fatal("Load() returned nil *Config with no error")
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

// missingRequiredEnv sets every required env var to a valid value, then
// clears just envVar — modeling a single required field being absent.
func missingRequiredEnv(envVar string) func(t *testing.T) {
	return func(t *testing.T) {
		clearConfigEnv(t)
		setValidRequiredEnv(t)
		t.Setenv(envVar, "")
	}
}

// malformedEnv sets every required env var to a valid value, then overrides
// envVar with a semantically invalid value.
func malformedEnv(envVar, value string) func(t *testing.T) {
	return func(t *testing.T) {
		clearConfigEnv(t)
		setValidRequiredEnv(t)
		t.Setenv(envVar, value)
	}
}

// optionalUnset sets every required env var to a valid value and leaves all
// optional vars unset, so Load() must apply their documented defaults.
func optionalUnset(t *testing.T) {
	clearConfigEnv(t)
	setValidRequiredEnv(t)
}

// wantField builds a check function asserting a single field, read via get,
// equals want.
func wantField(get func(*Config) any, want any) func(t *testing.T, cfg *Config) {
	return func(t *testing.T, cfg *Config) {
		if got := get(cfg); got != want {
			t.Errorf("field = %v, want %v", got, want)
		}
	}
}

func containsField(errMsg, field string) bool {
	return strings.Contains(errMsg, field)
}
