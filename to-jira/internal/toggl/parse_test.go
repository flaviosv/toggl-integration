package toggl

import "testing"

func TestParseDescription(t *testing.T) {
	cases := []struct {
		name     string
		desc     string
		wantKey  string
		wantText string
		wantOK   bool
	}{
		{
			name:     "valid tag with space before text",
			desc:     "[ABC-123] Did the thing",
			wantKey:  "ABC-123",
			wantText: "Did the thing",
			wantOK:   true,
		},
		{
			name:     "valid tag with no separating space",
			desc:     "[ABC-123]Did the thing",
			wantKey:  "ABC-123",
			wantText: "Did the thing",
			wantOK:   true,
		},
		{
			name:   "missing brackets",
			desc:   "ABC-123 Did the thing",
			wantOK: false,
		},
		{
			name:   "lowercase slug",
			desc:   "[abc-123] Did the thing",
			wantOK: false,
		},
		{
			name:   "no hyphen or number",
			desc:   "[ABC] Did the thing",
			wantOK: false,
		},
		{
			name:   "single-letter slug too short",
			desc:   "[A-123] Did the thing",
			wantOK: false,
		},
		{
			name:   "space between opening bracket and slug",
			desc:   "[ ABC-123] Did the thing",
			wantOK: false,
		},
		{
			name:   "space between slug and hyphen",
			desc:   "[ABC -123] Did the thing",
			wantOK: false,
		},
		{
			name:   "space between hyphen and number",
			desc:   "[ABC- 123] Did the thing",
			wantOK: false,
		},
		{
			name:   "space before closing bracket",
			desc:   "[ABC-123 ] Did the thing",
			wantOK: false,
		},
		{
			name:   "parentheses instead of brackets",
			desc:   "(ABC-123) Did the thing",
			wantOK: false,
		},
		{
			name:   "empty description",
			desc:   "",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, text, ok := ParseDescription(tc.desc)

			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (key=%q text=%q)", ok, tc.wantOK, key, text)
			}
			if !tc.wantOK {
				return
			}
			if key != tc.wantKey {
				t.Errorf("issueKey = %q, want %q", key, tc.wantKey)
			}
			if text != tc.wantText {
				t.Errorf("text = %q, want %q", text, tc.wantText)
			}
		})
	}
}
