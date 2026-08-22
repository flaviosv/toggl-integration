// Package routes registers the service's HTTP routes.
package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/flaviosv/toggl-integration/to-jira/internal/webhook"
)

// Routes registers the Toggl webhook endpoint on the v1 route group — not
// on app directly (applyr's AD-002 fix: dinherim registered routes on app,
// so its middleware chain never actually ran on them).
func Routes(v1 *gin.RouterGroup, h *webhook.Handler) {
	v1.POST("/webhooks/toggl", h.Receive)
}
