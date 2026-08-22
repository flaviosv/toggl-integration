package toggl

import "regexp"

var descriptionPattern = regexp.MustCompile(`^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$`)

// ParseDescription extracts the JIRA issue key and free-text description
// from a Toggl entry description tagged "[SLUG-NUMBER] text". ok is false
// when the description does not match the required format (TJ-02).
func ParseDescription(desc string) (issueKey, text string, ok bool) {
	m := descriptionPattern.FindStringSubmatch(desc)
	if m == nil {
		return "", "", false
	}
	return m[1] + "-" + m[2], m[3], true
}
