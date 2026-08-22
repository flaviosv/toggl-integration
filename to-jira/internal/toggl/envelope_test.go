package toggl

import (
	"encoding/json"
	"testing"
	"time"
)

// A well-formed envelope must unmarshal correctly, including its raw
// payload sub-document and nested metadata.
func TestParseEnvelope_ValidEnvelope(t *testing.T) {
	body := []byte(`{
		"event_id": "evt-1",
		"subscription_id": 42,
		"timestamp": "2026-08-22T10:00:00Z",
		"metadata": {"request_type": "time_entry.created"},
		"payload": {"id": 100, "description": "[ABC-123] did the thing"}
	}`)

	env, err := ParseEnvelope(body)
	if err != nil {
		t.Fatalf("ParseEnvelope() unexpected error: %v", err)
	}

	if env.EventID != "evt-1" {
		t.Errorf("EventID = %q, want %q", env.EventID, "evt-1")
	}
	if env.SubscriptionID != 42 {
		t.Errorf("SubscriptionID = %d, want 42", env.SubscriptionID)
	}
	wantTS := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	if !env.Timestamp.Equal(wantTS) {
		t.Errorf("Timestamp = %v, want %v", env.Timestamp, wantTS)
	}
	if env.Metadata.RequestType != "time_entry.created" {
		t.Errorf("Metadata.RequestType = %q, want %q", env.Metadata.RequestType, "time_entry.created")
	}

	var payload TimeEntryPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal env.Payload: %v", err)
	}
	if payload.ID != 100 {
		t.Errorf("payload.ID = %d, want 100", payload.ID)
	}
	if payload.Description == nil || *payload.Description != "[ABC-123] did the thing" {
		t.Errorf("payload.Description = %v, want %q", payload.Description, "[ABC-123] did the thing")
	}
}

// Malformed JSON must return an error, never panic.
func TestParseEnvelope_MalformedJSON(t *testing.T) {
	_, err := ParseEnvelope([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("ParseEnvelope() expected error for malformed JSON, got nil")
	}
}

// An envelope that omits optional/absent fields (payload, metadata) must
// still parse successfully with zero values, not error.
func TestParseEnvelope_MissingOptionalFields(t *testing.T) {
	body := []byte(`{"event_id": "evt-2", "subscription_id": 7}`)

	env, err := ParseEnvelope(body)
	if err != nil {
		t.Fatalf("ParseEnvelope() unexpected error: %v", err)
	}
	if env.EventID != "evt-2" {
		t.Errorf("EventID = %q, want %q", env.EventID, "evt-2")
	}
	if env.Metadata.RequestType != "" {
		t.Errorf("Metadata.RequestType = %q, want empty", env.Metadata.RequestType)
	}
	if env.Payload != nil {
		t.Errorf("Payload = %v, want nil", env.Payload)
	}
}
