// Package routes registers the service's HTTP routes.
package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/flaviosv/toggl-integration/to-jira/internal/webhook"
)

// Routes registers the Toggl webhook endpoint on group rather than directly
// on the root engine, so any middleware attached to group actually runs on
// this route (registering directly on the engine would bypass it).
func Routes(group *gin.RouterGroup, h *webhook.Handler) {
	group.POST("/webhooks/toggl", h.Receive)
}
