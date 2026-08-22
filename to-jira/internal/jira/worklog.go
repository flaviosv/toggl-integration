package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

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
		path := fmt.Sprintf("/rest/api/3/issue/%s/worklog?startAt=%d", url.PathEscape(issueKey), startAt)
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
