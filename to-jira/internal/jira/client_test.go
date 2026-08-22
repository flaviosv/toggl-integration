package jira

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewClient must wire the constructed Client to the given baseURL, so
// requests via do() land on the configured server.
func TestNewClient_UsesConfiguredBaseURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token", nil)

	resp, err := c.do(context.Background(), http.MethodGet, "/rest/api/3/issue/ABC-1/worklog", nil)
	if err != nil {
		t.Fatalf("do() error = %v", err)
	}
	resp.Body.Close()

	if gotPath != "/rest/api/3/issue/ABC-1/worklog" {
		t.Errorf("server saw path %q, want %q", gotPath, "/rest/api/3/issue/ABC-1/worklog")
	}
}

// do() must set HTTP Basic Auth for the configured email/token pair.
func TestDo_SetsBasicAuthHeader(t *testing.T) {
	const email = "user@example.com"
	const token = "secret-token"
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+token))

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, email, token, nil)

	resp, err := c.do(context.Background(), http.MethodGet, "/rest/api/3/issue/ABC-1/worklog", nil)
	if err != nil {
		t.Fatalf("do() error = %v", err)
	}
	resp.Body.Close()

	if gotAuth != wantAuth {
		t.Errorf("Authorization header = %q, want %q", gotAuth, wantAuth)
	}
}

// do() must return the raw response for a non-2xx status without turning it
// into an error — callers decide transient vs. permanent.
func TestDo_ReturnsNon2xxWithoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token", nil)

	resp, err := c.do(context.Background(), http.MethodGet, "/rest/api/3/issue/ABC-1/worklog", nil)
	if err != nil {
		t.Fatalf("do() error = %v, want nil (non-2xx is not a Go error at this layer)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
