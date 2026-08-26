package sync

import (
	"context"

	"github.com/flaviosv/toggl-integration/to-jira/internal/jira"
)

// fakeJiraClient is a JiraClient test double — the hard project constraint
// forbids live network calls in tests, so Processor is always exercised
// against this fake, never *jira.Client.
type fakeJiraClient struct {
	findResult *jira.Worklog
	findErr    error
	findCalls  int

	lastFindIssueKey string
	lastFindTogglID  string

	createResult *jira.Worklog
	createErr    error
	createCalls  int

	updateErr   error
	updateCalls int

	deleteErr   error
	deleteCalls int

	lastIssueKey  string
	lastWorklogID string
	lastInput     jira.WorklogInput
}

func (f *fakeJiraClient) FindWorklogByTogglID(_ context.Context, issueKey, togglID string) (*jira.Worklog, error) {
	f.findCalls++
	f.lastFindIssueKey = issueKey
	f.lastFindTogglID = togglID
	return f.findResult, f.findErr
}

func (f *fakeJiraClient) CreateWorklog(_ context.Context, issueKey string, in jira.WorklogInput) (*jira.Worklog, error) {
	f.createCalls++
	f.lastIssueKey = issueKey
	f.lastInput = in
	return f.createResult, f.createErr
}

func (f *fakeJiraClient) UpdateWorklog(_ context.Context, issueKey, worklogID string, in jira.WorklogInput) error {
	f.updateCalls++
	f.lastIssueKey = issueKey
	f.lastWorklogID = worklogID
	f.lastInput = in
	return f.updateErr
}

func (f *fakeJiraClient) DeleteWorklog(_ context.Context, issueKey, worklogID string) error {
	f.deleteCalls++
	f.lastIssueKey = issueKey
	f.lastWorklogID = worklogID
	return f.deleteErr
}
