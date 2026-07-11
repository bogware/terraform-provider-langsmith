// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxResponseBodySize is the maximum response body we'll read (10 MB).
const maxResponseBodySize = 10 * 1024 * 1024

// maxRetryAfterSecs is the maximum Retry-After we'll honor (60 seconds).
const maxRetryAfterSecs = 60

// Client is the LangSmith API client.
type Client struct {
	BaseURL     string
	APIKey      string
	WorkspaceID string
	UserAgent   string
	HTTPClient  *http.Client
	MaxRetries  int
}

// NewClient creates a new LangSmith API client.
func NewClient(baseURL, apiKey, workspaceID, userAgent string) *Client {
	return &Client{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		WorkspaceID: workspaceID,
		UserAgent:   userAgent,
		HTTPClient: &http.Client{
			Timeout:       120 * time.Second,
			CheckRedirect: dropAuthOnCrossHostRedirect,
		},
		MaxRetries: 5,
	}
}

// dropAuthOnCrossHostRedirect prevents the API key from being forwarded to a
// different host when the server issues a redirect. Go's default policy copies
// most headers to the redirect target; a redirect (accidental or malicious) to
// another host would otherwise leak X-API-Key and X-Tenant-Id. We follow the
// redirect but strip the credentials whenever the host changes.
func dropAuthOnCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if len(via) > 0 && req.URL.Host != via[0].URL.Host {
		req.Header.Del("X-API-Key")
		req.Header.Del("X-Tenant-Id")
	}
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body interface{}, result interface{}) error {
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
	}

	// Query parameters always arrive through the separate `query` argument and
	// are appended below, so a '?' or '#' in `path` can only come from an
	// unescaped, caller-supplied path segment (a repo handle, tag name, secret
	// key, ...). Left alone, such a character truncates the URL: a '#' turns the
	// rest of the path into a fragment, silently collapsing e.g.
	// DELETE /repos/-/{handle}/tags/{tag#...} down to the parent endpoint and
	// deleting the wrong object. Refuse it rather than send a mis-targeted request.
	if strings.ContainsAny(path, "?#") {
		return fmt.Errorf("invalid request path %q: contains a '?' or '#'; a path segment (such as a name, handle, or key) must not contain these characters", path)
	}

	reqURL := c.BaseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	var lastErr error
	skipBackoff := false
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 && !skipBackoff {
			wait := retryDelay(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		skipBackoff = false

		var bodyReader io.Reader
		if jsonBody != nil {
			bodyReader = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("X-API-Key", c.APIKey)
		if c.WorkspaceID != "" {
			req.Header.Set("X-Tenant-Id", c.WorkspaceID)
		}
		if c.UserAgent != "" {
			req.Header.Set("User-Agent", c.UserAgent)
		}
		if jsonBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("executing request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response body: %w", err)
			continue
		}

		if resp.StatusCode == 408 || resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = &APIError{
				Method:     method,
				Path:       path,
				StatusCode: resp.StatusCode,
				Body:       string(respBody),
			}
			// Honor Retry-After header on 429, then skip exponential backoff
			// for the next iteration to avoid double-waiting.
			if resp.StatusCode == 429 {
				retrySecs := 5 // default 5s backoff for 429 without Retry-After
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if parsed, parseErr := strconv.Atoi(ra); parseErr == nil {
						retrySecs = parsed
					}
				}
				if retrySecs > maxRetryAfterSecs {
					retrySecs = maxRetryAfterSecs
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(retrySecs) * time.Second):
				}
				skipBackoff = true
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &APIError{
				Method:     method,
				Path:       path,
				StatusCode: resp.StatusCode,
				Body:       string(respBody),
			}
		}

		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("unmarshaling response: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("request failed after %d retries: %w", c.MaxRetries+1, lastErr)
}

// retryDelay calculates exponential backoff with jitter: base * 2^(attempt-1) +/- 20%.
func retryDelay(attempt int) time.Duration {
	base := math.Pow(2, float64(attempt-1)) * float64(time.Second)
	jitter := base * 0.2 * (2*rand.Float64() - 1) // +/- 20%
	return time.Duration(base + jitter)
}

// Get sends an HTTP GET request.
func (c *Client) Get(ctx context.Context, path string, query url.Values, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, query, nil, result)
}

// Post sends an HTTP POST request.
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, nil, body, result)
}

// PostWithQuery sends an HTTP POST with query parameters.
func (c *Client) PostWithQuery(ctx context.Context, path string, query url.Values, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, query, body, result)
}

// Patch sends an HTTP PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPatch, path, nil, body, result)
}

// Put sends an HTTP PUT request.
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPut, path, nil, body, result)
}

// PutWithQuery sends an HTTP PUT with query parameters.
func (c *Client) PutWithQuery(ctx context.Context, path string, query url.Values, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPut, path, query, body, result)
}

// Delete sends an HTTP DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil, nil)
}

// DeleteWithQuery sends an HTTP DELETE with query parameters.
func (c *Client) DeleteWithQuery(ctx context.Context, path string, query url.Values) error {
	return c.doRequest(ctx, http.MethodDelete, path, query, nil, nil)
}

// DeleteWithBody sends an HTTP DELETE with a request body.
func (c *Client) DeleteWithBody(ctx context.Context, path string, body interface{}) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, body, nil)
}

// APIError represents an error response from the LangSmith API. Method and
// Path are populated where available so error messages identify which call
// failed; they may be empty on older code paths or constructed values.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Method != "" && e.Path != "" {
		return fmt.Sprintf("LangSmith %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("LangSmith API error (status %d): %s", e.StatusCode, e.Body)
}

// WithWorkspaceID returns a shallow copy of the Client with the WorkspaceID field
// replaced by the given value. The underlying http.Client and all other
// settings are shared with the original; only WorkspaceID differs. This is safe
// for concurrent use because http.Client is itself concurrency-safe.
func (c *Client) WithWorkspaceID(workspaceID string) *Client {
	clientCopy := *c
	clientCopy.WorkspaceID = workspaceID
	return &clientCopy
}

// IsNotFound checks whether the error is a 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}
