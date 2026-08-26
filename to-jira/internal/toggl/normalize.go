package toggl

import (
	"strconv"
	"time"
)

// Event is the normalized form sync.Processor consumes, built from a Toggl
// TimeEntryPayload.
type Event struct {
	TogglID     string
	Description string
	Duration    time.Duration
	StartedAt   time.Time
	HasStopped  bool
}

// NormalizeEntry converts a raw Toggl TimeEntryPayload into an Event.
//
// HasStopped is false when the entry is still running (Duration absent/negative, or
// Stop absent) — TJ-03. Description and StartedAt are still populated on
// the returned Event whenever the corresponding payload field is present,
// even when HasStopped is false: this lets a delete-payload caller (which only ever
// has a partial payload) still derive the issue key from Event.Description
// when the payload carries one (hypothesized delete shape A), while an
// Event.Description left empty distinguishes the non-derivable case
// (hypothesized delete shape B), which the caller maps to unsupported_delete.
func NormalizeEntry(p TimeEntryPayload) Event {
	var e Event
	e.TogglID = strconv.FormatInt(p.ID, 10)

	if p.Description != nil {
		e.Description = *p.Description
	}
	if p.Start != nil {
		e.StartedAt = *p.Start
	}

	if p.Duration == nil || *p.Duration < 0 || p.Stop == nil {
		return e
	}

	e.Duration = time.Duration(*p.Duration) * time.Second
	e.HasStopped = true
	return e
}
