package toggl

import (
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

// A complete entry (positive duration, Stop set) must normalize with
// HasStopped=true and every Event field correctly populated.
func TestNormalizeEntry_CompleteEntry(t *testing.T) {
	start := time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)
	stop := start.Add(30 * time.Minute)
	desc := "[ABC-123] Did the thing"
	p := TimeEntryPayload{
		ID:          100,
		Description: &desc,
		Duration:    ptr(int64(1800)),
		Start:       &start,
		Stop:        &stop,
	}

	e := NormalizeEntry(p)

	if e.TogglID != "100" {
		t.Errorf("TogglID = %q, want %q", e.TogglID, "100")
	}
	if e.Description != desc {
		t.Errorf("Description = %q, want %q", e.Description, desc)
	}
	if e.Duration != 30*time.Minute {
		t.Errorf("Duration = %v, want %v", e.Duration, 30*time.Minute)
	}
	if !e.StartedAt.Equal(start) {
		t.Errorf("StartedAt = %v, want %v", e.StartedAt, start)
	}
	if !e.HasStopped {
		t.Error("HasStopped = false, want true")
	}
}

// Running entries (negative duration, absent duration, or absent stop) must
// return HasStopped=false — TJ-03.
func TestNormalizeEntry_RunningEntry(t *testing.T) {
	desc := "[ABC-123] Did the thing"
	start := time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)
	stop := start.Add(30 * time.Minute)

	cases := []struct {
		name string
		p    TimeEntryPayload
	}{
		{
			name: "negative duration",
			p:    TimeEntryPayload{ID: 1, Description: &desc, Duration: ptr(int64(-1)), Start: &start, Stop: &stop},
		},
		{
			name: "nil stop",
			p:    TimeEntryPayload{ID: 2, Description: &desc, Duration: ptr(int64(1800)), Start: &start, Stop: nil},
		},
		{
			name: "nil duration",
			p:    TimeEntryPayload{ID: 3, Description: &desc, Duration: nil, Start: &start, Stop: &stop},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NormalizeEntry(tc.p)
			if e.HasStopped {
				t.Fatalf("HasStopped = true, want false for %s", tc.name)
			}
		})
	}
}

// Hypothesized delete shape A: Description present but no duration/stop.
// HasStopped is false (it looks like a running entry) but Event.Description must
// still be populated so the caller can derive the issue key from it.
func TestNormalizeEntry_DeleteShapeA_DescriptionPresentNoDuration(t *testing.T) {
	desc := "[ABC-123] Did the thing"
	p := TimeEntryPayload{ID: 100, Description: &desc}

	e := NormalizeEntry(p)

	if e.HasStopped {
		t.Fatalf("HasStopped = true, want false (event=%+v)", e)
	}
	if e.Description != desc {
		t.Errorf("Description = %q, want %q (must remain usable for issue derivation)", e.Description, desc)
	}
}

// Hypothesized delete shape B: Description nil entirely. HasStopped is false and
// Event.Description is left empty — the distinguishable signal the caller
// maps to unsupported_delete.
func TestNormalizeEntry_DeleteShapeB_DescriptionNil(t *testing.T) {
	p := TimeEntryPayload{ID: 100}

	e := NormalizeEntry(p)

	if e.HasStopped {
		t.Fatalf("HasStopped = true, want false (event=%+v)", e)
	}
	if e.Description != "" {
		t.Errorf("Description = %q, want empty (non-derivable payload)", e.Description)
	}
}

// Zero duration is a boundary that marks a stopped (completed) entry, not a
// running entry. Must return HasStopped=true.
func TestNormalizeEntry_ZeroDuration(t *testing.T) {
	start := time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)
	stop := start
	desc := "[ABC-123] Did the thing"
	p := TimeEntryPayload{
		ID:          100,
		Description: &desc,
		Duration:    ptr(int64(0)),
		Start:       &start,
		Stop:        &stop,
	}

	e := NormalizeEntry(p)

	if !e.HasStopped {
		t.Errorf("HasStopped = false, want true")
	}
	if e.Duration != 0 {
		t.Errorf("Duration = %v, want 0", e.Duration)
	}
}
