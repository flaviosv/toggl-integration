// Package webhook is the HTTP entrypoint for Toggl webhook deliveries: it
// verifies the request's HMAC signature, parses the event envelope, and
// dispatches to sync.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/flaviosv/toggl-integration/to-jira/internal/sync"
	"github.com/flaviosv/toggl-integration/to-jira/internal/toggl"
)

const (
	signatureHeader = "X-Webhook-Signature-256"
	signaturePrefix = "sha256="
)

// Handler is the HTTP entrypoint for POST /webhooks/toggl (TJ-01).
type Handler struct {
	secret string
	p      *sync.Processor
	logger *slog.Logger
}

// NewHandler constructs a Handler.
func NewHandler(secret string, p *sync.Processor, logger *slog.Logger) *Handler {
	return &Handler{secret: secret, p: p, logger: logger}
}

// Receive verifies the request's HMAC-SHA256 signature against the raw body
// (TJ-01), then parses the envelope and dispatches to sync by event type.
//
// The raw body MUST be read via io.ReadAll before any JSON binding — gin's
// ShouldBindJSON would consume the body stream first, making it impossible
// to compute the HMAC over the exact bytes Toggl signed (design.md's
// flagged raw-body-vs-binding gotcha). json.Unmarshal is used directly on
// the same byte slice instead.
func (h *Handler) Receive(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)

	if !verifySignature(h.secret, body, c.GetHeader(signatureHeader)) {
		c.Status(http.StatusUnauthorized)
		return
	}

	env, err := toggl.ParseEnvelope(body)
	if err != nil {
		h.logger.Warn("webhook: malformed envelope", "error", err)
		c.Status(http.StatusOK)
		return
	}

	var result sync.Result
	switch env.Metadata.RequestType {
	case "time_entry.created", "time_entry.updated":
		var payload toggl.TimeEntryPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			h.logger.Warn("webhook: malformed time entry payload", "error", err)
			c.Status(http.StatusOK)
			return
		}
		event, _ := toggl.NormalizeEntry(payload)
		result = h.p.Process(c.Request.Context(), event)
	case "time_entry.deleted":
		result = h.p.ProcessDelete(c.Request.Context(), env)
	default:
		c.Status(http.StatusOK)
		return
	}

	c.Status(result.HTTPStatus)
}

// verifySignature reports whether headerSig is a valid "sha256=<hex>"
// HMAC-SHA256 of rawBody under secret (TJ-01) — Toggl's documented
// X-Webhook-Signature-256 format.
func verifySignature(secret string, rawBody []byte, headerSig string) bool {
	want, ok := strings.CutPrefix(headerSig, signaturePrefix)
	if !ok {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	got := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(got), []byte(want))
}
