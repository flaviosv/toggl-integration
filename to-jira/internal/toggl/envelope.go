package toggl

import (
	"encoding/json"
	"fmt"
	"time"
)

// WebhookEnvelope is the outer shape of every Toggl webhook delivery.
type WebhookEnvelope struct {
	EventID        string          `json:"event_id"`
	SubscriptionID int64           `json:"subscription_id"`
	Timestamp      time.Time       `json:"timestamp"`
	Metadata       EventMetadata   `json:"metadata"`
	Payload        json.RawMessage `json:"payload"`
}

// EventMetadata carries the entity.action request type (e.g. "time_entry.created").
type EventMetadata struct {
	RequestType string `json:"request_type"`
}

// TimeEntryPayload is the inner Toggl time entry payload. Fields are
// pointers because the delete-event payload shape is unverified (see
// design.md's Risks) — a nil field distinguishes "absent" from a legitimate
// zero value.
type TimeEntryPayload struct {
	ID          int64      `json:"id"`
	Description *string    `json:"description"`
	Duration    *int64     `json:"duration"`
	Start       *time.Time `json:"start"`
	Stop        *time.Time `json:"stop"`
}

// ParseEnvelope unmarshals a raw webhook request body into a WebhookEnvelope.
func ParseEnvelope(body []byte) (WebhookEnvelope, error) {
	var env WebhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return WebhookEnvelope{}, fmt.Errorf("toggl: parse envelope: %w", err)
	}
	return env, nil
}
