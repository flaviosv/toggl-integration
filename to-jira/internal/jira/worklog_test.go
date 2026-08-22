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
	page0 := worklogFixture("1", "1")
	page1 := worklogFixture("2", "42")
	var requestedStartAt []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		requestedStartAt = append(requestedStartAt, startAt)

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
}

// FindWorklogByTogglID must return a *TransientError on a 5xx response.
func TestFindWorklogByTogglID_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	_, err := c.FindWorklogByTogglID(context.Background(), "ABC-1", "42")

	var transientErr *TransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("err = %v (%T), want *TransientError", err, err)
	}
}

// CreateWorklog must send timeSpentSeconds, started, and comment (ADF) in
// the request body, and decode the created worklog from the response.
func TestCreateWorklog_SendsCorrectBody(t *testing.T) {
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

// CreateWorklog must return a *TransientError on a 5xx response.
func TestCreateWorklog_TransientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	_, err := c.CreateWorklog(context.Background(), "ABC-1", WorklogInput{})

	var transientErr *TransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("err = %v (%T), want *TransientError", err, err)
	}
}

// CreateWorklog must return a *PermanentError on a 4xx response (e.g.
// unknown issue key).
func TestCreateWorklog_PermanentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "user@example.com", "token", nil)

	_, err := c.CreateWorklog(context.Background(), "UNKNOWN-1", WorklogInput{})

	var permanentErr *PermanentError
	if !errors.As(err, &permanentErr) {
		t.Fatalf("err = %v (%T), want *PermanentError", err, err)
	}
}

// CreateWorklog must return a *TransientError on a network-level failure
// (server unreachable), distinct from a permanent 4xx.
func TestCreateWorklog_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := NewClient(srv.URL, "user@example.com", "token", nil)
	srv.Close()

	_, err := c.CreateWorklog(context.Background(), "ABC-1", WorklogInput{})

	var transientErr *TransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("err = %v (%T), want *TransientError", err, err)
	}
}
