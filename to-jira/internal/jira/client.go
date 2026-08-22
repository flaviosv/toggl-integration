package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is a JIRA Cloud REST API v3 client authenticated via HTTP Basic
// Auth (email + API token).
type Client struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
}

// NewClient constructs a Client. hc defaults to http.DefaultClient when nil.
func NewClient(baseURL, email, apiToken string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		email:      email,
		apiToken:   apiToken,
		httpClient: hc,
	}
}

// do issues an authenticated request against path (relative to baseURL).
// body, when non-nil, is JSON-encoded as the request body. do returns the
// raw response and error as-is — it never inspects the status code, leaving
// transient-vs-permanent classification to the caller.
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("jira: encode request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("jira: build request: %w", err)
	}
	req.SetBasicAuth(c.email, c.apiToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}
