package jira

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func worklogFixture(id, togglID string) Worklog {
	return Worklog{ID: id, Comment: BuildComment(togglID, "did the thing")}
}

// FindWorklogByTogglID must return the matching worklog when present.
func TestFindWorklogByTogglID_Found(t *testing.T) {
	t.Parallel()
	want := worklogFixture("100028", "42")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := pageOfWorklogs{StartAt: 0, MaxResults: 10, Total: 1, Worklogs: []Worklog{want}}
		json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	got, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("got = nil, want a matching worklog")
	}
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
}

// FindWorklogByTogglID must return nil, nil when no worklog matches.
func TestFindWorklogByTogglID_NotFound(t *testing.T) {
	t.Parallel()
	other := worklogFixture("1", "99")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := pageOfWorklogs{StartAt: 0, MaxResults: 10, Total: 1, Worklogs: []Worklog{other}}
		json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	got, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
}

// FindWorklogByTogglID must follow pagination (startAt) across multiple
// pages until either a match is found or total is exhausted.
func TestFindWorklogByTogglID_Paginated(t *testing.T) {
	t.Parallel()
	page0 := worklogFixture("1", "1")
	page1 := worklogFixture("2", "42")
	var requestedStartAt []string
	var requestedMaxResults []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		maxResults := r.URL.Query().Get("maxResults")
		requestedStartAt = append(requestedStartAt, startAt)
		requestedMaxResults = append(requestedMaxResults, maxResults)

		var page pageOfWorklogs
		if startAt == "0" {
			page = pageOfWorklogs{StartAt: 0, MaxResults: 1, Total: 2, Worklogs: []Worklog{page0}}
		} else {
			page = pageOfWorklogs{StartAt: 1, MaxResults: 1, Total: 2, Worklogs: []Worklog{page1}}
		}
		json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	got, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got == nil || got.ID != "2" {
		t.Fatalf("got = %+v, want worklog ID 2", got)
	}
	if len(requestedStartAt) != 2 || requestedStartAt[0] != "0" || requestedStartAt[1] != "1" {
		t.Errorf("requestedStartAt = %v, want [0 1]", requestedStartAt)
	}
	if len(requestedMaxResults) != 2 || requestedMaxResults[0] != "100" || requestedMaxResults[1] != "100" {
		t.Errorf("requestedMaxResults = %v, want [100 100]", requestedMaxResults)
	}
}

// UpdateWorklog must send timeSpentSeconds, started, and comment (ADF) as
// the PUT body against the correct worklogID path.
func TestUpdateWorklog_SendsCorrectBody(t *testing.T) {
	t.Parallel()
	started := time.Date(2021, 1, 17, 12, 34, 0, 0, time.UTC)
	in := WorklogInput{TimeSpentSeconds: 3600, Started: started, Comment: BuildComment("42", "updated")}

	var gotMethod, gotPath string
	var gotBody worklogRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Worklog{ID: "100028", Comment: in.Comment})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	err := c.UpdateWorklog(context.Background(), "ABC-1", "100028", in)

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/rest/api/3/issue/ABC-1/worklog/100028" {
		t.Errorf("path = %q, want %q", gotPath, "/rest/api/3/issue/ABC-1/worklog/100028")
	}
	if gotBody.TimeSpentSeconds != 3600 {
		t.Errorf("TimeSpentSeconds = %d, want 3600", gotBody.TimeSpentSeconds)
	}
	if id, ok := ExtractTogglID(gotBody.Comment); !ok || id != "42" {
		t.Errorf("Comment TogglID = %q (ok=%v), want 42", id, ok)
	}
}


// DeleteWorklog must return nil on a successful delete against a fake
// server.
func TestDeleteWorklog_Success(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	err := c.DeleteWorklog(context.Background(), "ABC-1", "100028")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/rest/api/3/issue/ABC-1/worklog/100028" {
		t.Errorf("path = %q, want %q", gotPath, "/rest/api/3/issue/ABC-1/worklog/100028")
	}
}


// CreateWorklog must send timeSpentSeconds, started, and comment (ADF) in
// the request body, and decode the created worklog from the response.
func TestCreateWorklog_SendsCorrectBody(t *testing.T) {
	t.Parallel()
	started := time.Date(2021, 1, 17, 12, 34, 0, 0, time.UTC)
	in := WorklogInput{TimeSpentSeconds: 1800, Started: started, Comment: BuildComment("42", "did the thing")}

	var gotMethod, gotPath string
	var gotBody worklogRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Worklog{ID: "100028", Comment: in.Comment})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	got, err := c.CreateWorklog(context.Background(), "ABC-1", in)

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/rest/api/3/issue/ABC-1/worklog" {
		t.Errorf("path = %q, want %q", gotPath, "/rest/api/3/issue/ABC-1/worklog")
	}
	if gotBody.TimeSpentSeconds != 1800 {
		t.Errorf("TimeSpentSeconds = %d, want 1800", gotBody.TimeSpentSeconds)
	}
	if gotBody.Started != "2021-01-17T12:34:00.000+0000" {
		t.Errorf("Started = %q, want %q", gotBody.Started, "2021-01-17T12:34:00.000+0000")
	}
	if id, ok := ExtractTogglID(gotBody.Comment); !ok || id != "42" {
		t.Errorf("Comment TogglID = %q (ok=%v), want 42", id, ok)
	}
	if got.ID != "100028" {
		t.Errorf("returned Worklog.ID = %q, want %q", got.ID, "100028")
	}
}


// CreateWorklog must return a *TransientError on a network-level failure
// (server unreachable), distinct from a permanent 4xx.
func TestCreateWorklog_NetworkError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := NewClient(srv.URL, "user@example.com", "token", nil)
	srv.Close()

	_, err := c.CreateWorklog(context.Background(), "ABC-1", WorklogInput{})

	var transientErr *TransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("err = %v (%T), want *TransientError", err, err)
	}
}

// FindWorklogByTogglID must return an error when the response body is not
// valid JSON, even when the status is 2xx.
func TestFindWorklogByTogglID_MalformedJSONBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	_, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")

	if err == nil {
		t.Fatal("err = nil, want non-nil error")
	}
}

// CreateWorklog must return an error when the response body is not valid JSON,
// even when the status is 2xx (201).
func TestCreateWorklog_MalformedJSONBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	_, err := c.CreateWorklog(context.Background(), "ABC-1", WorklogInput{})

	if err == nil {
		t.Fatal("err = nil, want non-nil error")
	}
}

// FindWorklogByTogglID must return a *TransientError on a network-level failure
// (server unreachable).
func TestFindWorklogByTogglID_NetworkError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := NewClient(srv.URL, "user@example.com", "token", nil)
	srv.Close()

	_, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")

	var transientErr *TransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("err = %v (%T), want *TransientError", err, err)
	}
}

// UpdateWorklog must return a *TransientError on a network-level failure
// (server unreachable).
func TestUpdateWorklog_NetworkError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := NewClient(srv.URL, "user@example.com", "token", nil)
	srv.Close()

	err := c.UpdateWorklog(context.Background(), "ABC-1", "100028", WorklogInput{})

	var transientErr *TransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("err = %v (%T), want *TransientError", err, err)
	}
}

// DeleteWorklog must return a *TransientError on a network-level failure
// (server unreachable).
func TestDeleteWorklog_NetworkError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := NewClient(srv.URL, "user@example.com", "token", nil)
	srv.Close()

	err := c.DeleteWorklog(context.Background(), "ABC-1", "100028")

	var transientErr *TransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("err = %v (%T), want *TransientError", err, err)
	}
}

// TestWorklogMethods_ClassifyErrorsByStatus tests error classification across
// all worklog methods for various HTTP status codes.
func TestWorklogMethods_ClassifyErrorsByStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		statusCode    int
		call          func(c *Client) error
		wantTransient bool
	}{
		// FindWorklogByTogglID cases
		{
			name:       "FindWorklogByTogglID_Transient500",
			statusCode: http.StatusInternalServerError,
			call: func(c *Client) error {
				_, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")
				return err
			},
			wantTransient: true,
		},
		{
			name:       "FindWorklogByTogglID_Permanent404",
			statusCode: http.StatusNotFound,
			call: func(c *Client) error {
				_, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")
				return err
			},
			wantTransient: false,
		},
		// CreateWorklog cases
		{
			name:       "CreateWorklog_Transient503",
			statusCode: http.StatusServiceUnavailable,
			call: func(c *Client) error {
				_, err := c.CreateWorklog(context.Background(), "ABC-1", WorklogInput{})
				return err
			},
			wantTransient: true,
		},
		{
			name:       "CreateWorklog_Permanent404",
			statusCode: http.StatusNotFound,
			call: func(c *Client) error {
				_, err := c.CreateWorklog(context.Background(), "UNKNOWN-1", WorklogInput{})
				return err
			},
			wantTransient: false,
		},
		// UpdateWorklog cases
		{
			name:       "UpdateWorklog_Transient503",
			statusCode: http.StatusServiceUnavailable,
			call: func(c *Client) error {
				return c.UpdateWorklog(context.Background(), "ABC-1", "100028", WorklogInput{})
			},
			wantTransient: true,
		},
		{
			name:       "UpdateWorklog_Permanent404",
			statusCode: http.StatusNotFound,
			call: func(c *Client) error {
				return c.UpdateWorklog(context.Background(), "ABC-1", "unknown-worklog", WorklogInput{})
			},
			wantTransient: false,
		},
		// DeleteWorklog cases
		{
			name:       "DeleteWorklog_Transient502",
			statusCode: http.StatusBadGateway,
			call: func(c *Client) error {
				return c.DeleteWorklog(context.Background(), "ABC-1", "100028")
			},
			wantTransient: true,
		},
		{
			name:       "DeleteWorklog_Permanent404",
			statusCode: http.StatusNotFound,
			call: func(c *Client) error {
				return c.DeleteWorklog(context.Background(), "ABC-1", "unknown-worklog")
			},
			wantTransient: false,
		},
		// Rate limit case
		{
			name:       "CreateWorklog_Transient429",
			statusCode: http.StatusTooManyRequests,
			call: func(c *Client) error {
				_, err := c.CreateWorklog(context.Background(), "ABC-1", WorklogInput{})
				return err
			},
			wantTransient: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()
			c := NewClient(srv.URL, "user@example.com", "token", nil)

			err := tc.call(c)

			if tc.wantTransient {
				var transientErr *TransientError
				if !errors.As(err, &transientErr) {
					t.Fatalf("err = %v (%T), want *TransientError", err, err)
				}
			} else {
				var permanentErr *PermanentError
				if !errors.As(err, &permanentErr) {
					t.Fatalf("err = %v (%T), want *PermanentError", err, err)
				}
			}
		})
	}
}
