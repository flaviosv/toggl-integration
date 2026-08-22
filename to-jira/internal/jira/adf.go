// Package jira implements a JIRA Cloud REST API v3 client for worklog CRUD
// and Atlassian Document Format (ADF) comment marshaling.
package jira

import (
	"fmt"
	"regexp"
	"time"
)

// WorklogInput is the request-scoped payload for creating or updating a
// JIRA worklog.
type WorklogInput struct {
	TimeSpentSeconds int64
	Started          time.Time
	Comment          ADFDocument
}

// Worklog is the subset of a JIRA worklog response this package cares about.
type Worklog struct {
	ID      string      `json:"id"`
	Comment ADFDocument `json:"comment"`
}

// ADFDocument is a minimal Atlassian Document Format document, matching the
// shape required for worklog comments: a single "doc" root with paragraph
// and text nodes.
type ADFDocument struct {
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Content []ADFNode `json:"content"`
}

// ADFNode is a single ADF node — either a container (e.g. "paragraph", via
// Content) or a leaf (e.g. "text", via Text).
type ADFNode struct {
	Type    string    `json:"type"`
	Content []ADFNode `json:"content,omitempty"`
	Text    string    `json:"text,omitempty"`
}

const togglIDPrefix = "TogglID:"

var togglIDPattern = regexp.MustCompile(`^\[TogglID:([^\]]+)\]`)

// BuildComment produces a single-paragraph/single-text-run ADF document
// whose text is "[TogglID:<togglID>] <text>" — the marker ExtractTogglID
// looks for.
func BuildComment(togglID, text string) ADFDocument {
	body := fmt.Sprintf("[%s%s] %s", togglIDPrefix, togglID, text)
	return ADFDocument{
		Type:    "doc",
		Version: 1,
		Content: []ADFNode{
			{
				Type: "paragraph",
				Content: []ADFNode{
					{Type: "text", Text: body},
				},
			},
		},
	}
}

// ExtractTogglID reads the TogglID marker from the first text run of the
// first paragraph node of doc. ok is false when doc has no such marker —
// either because it was never written by BuildComment, or because it was
// structurally altered (e.g. manually edited via the JIRA UI). This is a
// documented simplification (design.md's Risks & Concerns): only the first
// text run of the first paragraph is ever inspected.
func ExtractTogglID(doc ADFDocument) (string, bool) {
	if len(doc.Content) == 0 {
		return "", false
	}
	para := doc.Content[0]
	if para.Type != "paragraph" || len(para.Content) == 0 {
		return "", false
	}
	textNode := para.Content[0]
	if textNode.Type != "text" {
		return "", false
	}
	m := togglIDPattern.FindStringSubmatch(textNode.Text)
	if m == nil {
		return "", false
	}
	return m[1], true
}
