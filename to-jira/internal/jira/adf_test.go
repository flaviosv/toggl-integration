package jira

import "testing"

// BuildComment must produce a single-paragraph/single-text-run ADF document
// whose text is "[TogglID:<id>] <text>".
func TestBuildComment_ProducesExpectedShape(t *testing.T) {
	doc := BuildComment("42", "Did the thing")

	if doc.Type != "doc" {
		t.Errorf("Type = %q, want %q", doc.Type, "doc")
	}
	if doc.Version != 1 {
		t.Errorf("Version = %d, want %d", doc.Version, 1)
	}
	if len(doc.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(doc.Content))
	}
	para := doc.Content[0]
	if para.Type != "paragraph" {
		t.Errorf("Content[0].Type = %q, want %q", para.Type, "paragraph")
	}
	if len(para.Content) != 1 {
		t.Fatalf("len(Content[0].Content) = %d, want 1", len(para.Content))
	}
	text := para.Content[0]
	if text.Type != "text" {
		t.Errorf("Content[0].Content[0].Type = %q, want %q", text.Type, "text")
	}
	want := "[TogglID:42] Did the thing"
	if text.Text != want {
		t.Errorf("Content[0].Content[0].Text = %q, want %q", text.Text, want)
	}
}

// ExtractTogglID must round-trip a BuildComment output back to the original
// TogglID.
func TestExtractTogglID_RoundTrip(t *testing.T) {
	doc := BuildComment("42", "Did the thing")

	id, ok := ExtractTogglID(doc)

	if !ok {
		t.Fatal("ok = false, want true")
	}
	if id != "42" {
		t.Errorf("id = %q, want %q", id, "42")
	}
}

// ExtractTogglID must return ok=false on a comment without the marker, and
// on structurally different (e.g. manually-edited) ADF documents.
func TestExtractTogglID_NoMarkerOrStructurallyDifferent(t *testing.T) {
	cases := []struct {
		name string
		doc  ADFDocument
	}{
		{
			name: "no marker",
			doc: ADFDocument{Type: "doc", Version: 1, Content: []ADFNode{
				{Type: "paragraph", Content: []ADFNode{{Type: "text", Text: "just a comment"}}},
			}},
		},
		{
			name: "empty content",
			doc:  ADFDocument{Type: "doc", Version: 1, Content: nil},
		},
		{
			name: "first node not a paragraph",
			doc: ADFDocument{Type: "doc", Version: 1, Content: []ADFNode{
				{Type: "heading", Content: []ADFNode{{Type: "text", Text: "[TogglID:42] heading"}}},
			}},
		},
		{
			name: "paragraph's first child not text",
			doc: ADFDocument{Type: "doc", Version: 1, Content: []ADFNode{
				{Type: "paragraph", Content: []ADFNode{{Type: "hardBreak"}}},
			}},
		},
		{
			name: "paragraph with empty content",
			doc: ADFDocument{Type: "doc", Version: 1, Content: []ADFNode{
				{Type: "paragraph", Content: nil},
			}},
		},
		{
			name: "marker not at start of text run",
			doc: ADFDocument{Type: "doc", Version: 1, Content: []ADFNode{
				{Type: "paragraph", Content: []ADFNode{{Type: "text", Text: "edited [TogglID:42] moved"}}},
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := ExtractTogglID(tc.doc)
			if ok {
				t.Errorf("ok = true, want false for %s", tc.name)
			}
		})
	}
}
