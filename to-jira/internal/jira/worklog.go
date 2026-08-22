package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// jiraTimeLayout formats time.Time as JIRA's worklog "started" field expects
// (e.g. "2021-01-17T12:34:00.000+0000") — millisecond precision, no colon in
// the UTC offset.
const jiraTimeLayout = "2006-01-02T15:04:05.000-0700"

const findWorklogPageSize = 100

// worklogRequestBody is the JIRA v3 request shape shared by CreateWorklog
// and UpdateWorklog.
type worklogRequestBody struct {
	TimeSpentSeconds int64       `json:"timeSpentSeconds"`
	Started          string      `json:"started"`
	Comment          ADFDocument `json:"comment"`
}

func newWorklogRequestBody(in WorklogInput) worklogRequestBody {
	return worklogRequestBody{
		TimeSpentSeconds: in.TimeSpentSeconds,
		Started:          in.Started.Format(jiraTimeLayout),
		Comment:          in.Comment,
	}
}

// TransientError wraps a JIRA API failure that is likely to succeed on
// retry: a network-level error, a 5xx response, or a 429 (rate limit).
type TransientError struct {
	StatusCode int
	Err        error
}

func (e *TransientError) Error() string { return e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

// PermanentError wraps a JIRA API failure that will not succeed on retry
// without a change to the request itself (e.g. 400, 404).
type PermanentError struct {
	StatusCode int
	Err        error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// classifyStatus inspects resp's status code and returns nil for 2xx, a
// *TransientError for 5xx/429, or a *PermanentError for any other non-2xx.
func classifyStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	err := fmt.Errorf("jira: %s %s: status %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, string(body))
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return &TransientError{StatusCode: resp.StatusCode, Err: err}
	}
	return &PermanentError{StatusCode: resp.StatusCode, Err: err}
}

// pageOfWorklogs is the JIRA v3 response shape for GET .../worklog.
type pageOfWorklogs struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}

// FindWorklogByTogglID lists the worklogs on issueKey and returns the first
// one whose ADF comment carries the given togglID marker. Returns (nil, nil)
// when no worklog matches — JIRA has no server-side comment search, so this
// always requires a full client-side scan (paginated).
func (c *Client) FindWorklogByTogglID(ctx context.Context, issueKey, togglID string) (*Worklog, error) {
	startAt := 0
	for {
		path := fmt.Sprintf("/rest/api/3/issue/%s/worklog?startAt=%d&maxResults=%d", url.PathEscape(issueKey), startAt, findWorklogPageSize)
		resp, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, &TransientError{Err: fmt.Errorf("jira: list worklogs: %w", err)}
		}

		if statusErr := classifyStatus(resp); statusErr != nil {
			resp.Body.Close()
			return nil, statusErr
		}

		var page pageOfWorklogs
		decodeErr := json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("jira: decode worklog page: %w", decodeErr)
		}

		for _, w := range page.Worklogs {
			if id, ok := ExtractTogglID(w.Comment); ok && id == togglID {
				match := w
				return &match, nil
			}
		}

		startAt += len(page.Worklogs)
		if len(page.Worklogs) == 0 || startAt >= page.Total {
			return nil, nil
		}
	}
}

// CreateWorklog adds a new worklog to issueKey via POST .../worklog (TJ-05).
func (c *Client) CreateWorklog(ctx context.Context, issueKey string, in WorklogInput) (*Worklog, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/worklog", url.PathEscape(issueKey))
	resp, err := c.do(ctx, http.MethodPost, path, newWorklogRequestBody(in))
	if err != nil {
		return nil, &TransientError{Err: fmt.Errorf("jira: create worklog: %w", err)}
	}
	defer resp.Body.Close()

	if statusErr := classifyStatus(resp); statusErr != nil {
		return nil, statusErr
	}

	var created Worklog
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("jira: decode created worklog: %w", err)
	}
	return &created, nil
}

// UpdateWorklog replaces worklogID's duration/comment on issueKey via
// PUT .../worklog/{worklogId} (TJ-06).
func (c *Client) UpdateWorklog(ctx context.Context, issueKey, worklogID string, in WorklogInput) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/worklog/%s", url.PathEscape(issueKey), url.PathEscape(worklogID))
	resp, err := c.do(ctx, http.MethodPut, path, newWorklogRequestBody(in))
	if err != nil {
		return &TransientError{Err: fmt.Errorf("jira: update worklog: %w", err)}
	}
	defer resp.Body.Close()

	return classifyStatus(resp)
}

// DeleteWorklog removes worklogID from issueKey via
// DELETE .../worklog/{worklogId} (TJ-10).
func (c *Client) DeleteWorklog(ctx context.Context, issueKey, worklogID string) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/worklog/%s", url.PathEscape(issueKey), url.PathEscape(worklogID))
	resp, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return &TransientError{Err: fmt.Errorf("jira: delete worklog: %w", err)}
	}
	defer resp.Body.Close()

	return classifyStatus(resp)
}
