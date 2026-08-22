package jira

import (
	"log/slog"
	"time"
)

// tokenExpiryWarnWindow is how far ahead of the configured expiry date
// WarnIfTokenExpiringSoon starts warning.
const tokenExpiryWarnWindow = 14 * 24 * time.Hour

// WarnIfTokenExpiringSoon logs a warning when configuredExpiry is within 14
// days (or already past), an informational note when configuredExpiry is
// nil (expiry isn't being tracked), and nothing when it's comfortably far
// away (TJ-15).
func (c *Client) WarnIfTokenExpiringSoon(configuredExpiry *time.Time, logger *slog.Logger) {
	if configuredExpiry == nil {
		logger.Info("jira: API token expiry not tracked (JIRA_API_TOKEN_EXPIRES_AT not set)")
		return
	}

	if time.Until(*configuredExpiry) <= tokenExpiryWarnWindow {
		logger.Warn("jira: API token expiring soon", "expires_at", configuredExpiry.Format(time.RFC3339))
	}
}
