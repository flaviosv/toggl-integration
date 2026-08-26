package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewClient must wire the constructed Client to the given baseURL, so
// requests via do() land on the configured server.
func TestNewClient_UsesConfiguredBaseURL(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// NewClient must apply defaultClientTimeout when hc is nil.
func TestNewClient_NilHTTPClient_AppliesDefaultTimeout(t *testing.T) {
	t.Parallel()
	c := NewClient("https://example.atlassian.net", "user@example.com", "token", nil)

	if c.httpClient.Timeout != defaultClientTimeout {
		t.Errorf("Timeout = %v, want %v", c.httpClient.Timeout, defaultClientTimeout)
	}
	if c.httpClient.Timeout <= 0 {
		t.Errorf("Timeout = %v, want > 0", c.httpClient.Timeout)
	}
}

// do() must set Content-Type header and JSON-encode the body when body is non-nil.
func TestDo_NonNilBody_SetsContentTypeAndMarshalsJSON(t *testing.T) {
	t.Parallel()
	type testBody struct {
		Foo string `json:"foo"`
	}

	var gotContentType string
	var gotBody testBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@example.com", "token", nil)

	resp, err := c.do(context.Background(), http.MethodPost, "/rest/api/3/issue", testBody{Foo: "bar"})
	if err != nil {
		t.Fatalf("do() error = %v", err)
	}
	resp.Body.Close()

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
	if gotBody.Foo != "bar" {
		t.Errorf("body.Foo = %q, want %q", gotBody.Foo, "bar")
	}
}
