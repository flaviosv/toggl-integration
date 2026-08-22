package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

// tokenExpiryDateLayout is the format JIRA_API_TOKEN_EXPIRES_AT is parsed
// with: a plain calendar date (no time-of-day), since the value is copied
// by hand from the date Atlassian shows at API token creation.
const tokenExpiryDateLayout = "2006-01-02"

type Config struct {
	TogglWebhookSecret string `validate:"required"`

	Jira JiraConfig

	DryRun                   bool
	OtelExporterOTLPEndpoint string
	Port                     int `validate:"required,min=1,max=65535"`
}

type JiraConfig struct {
	BaseURL         string `validate:"required"`
	Email           string `validate:"required"`
	APIToken        string `validate:"required"`
	APITokenExpires *time.Time
}

// Optional environment variables fall back to these defaults when absent.
// Variables with no default below are required: Load() fails when they are missing.
const (
	defaultDryRun = false
	defaultPort   = 8080
)

func Load() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config
	var errs []error

	cfg.TogglWebhookSecret = os.Getenv("TOGGL_WEBHOOK_SECRET")

	cfg.Jira.BaseURL = os.Getenv("JIRA_BASE_URL")
	cfg.Jira.Email = os.Getenv("JIRA_EMAIL")
	cfg.Jira.APIToken = os.Getenv("JIRA_API_TOKEN")
	cfg.Jira.APITokenExpires = parseDateEnv("JIRA_API_TOKEN_EXPIRES_AT", &errs)

	cfg.DryRun = parseBoolEnv("DRY_RUN", defaultDryRun, &errs)
	cfg.OtelExporterOTLPEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	cfg.Port = parseIntEnv("PORT", defaultPort, &errs)

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	if err := validator.New().Struct(&cfg); err != nil {
		return nil, describeValidationError(err)
	}

	return &cfg, nil
}

func parseIntEnv(key string, def int, errs *[]error) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("config: invalid value for %s: %q is not a valid integer", key, v))
		return 0
	}
	return n
}

func parseBoolEnv(key string, def bool, errs *[]error) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("config: invalid value for %s: %q is not a valid boolean", key, v))
		return false
	}
	return b
}

func parseDateEnv(key string, errs *[]error) *time.Time {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return nil
	}
	t, err := time.Parse(tokenExpiryDateLayout, v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("config: invalid value for %s: %q is not a valid date (want YYYY-MM-DD)", key, v))
		return nil
	}
	return &t
}

func describeValidationError(err error) error {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		fieldErrs := make([]error, 0, len(verrs))
		for _, fe := range verrs {
			fieldErrs = append(fieldErrs, fmt.Errorf("config: %s is %s (got %q)", fe.Namespace(), fe.Tag(), fe.Value()))
		}
		return errors.Join(fieldErrs...)
	}
	return err
}
