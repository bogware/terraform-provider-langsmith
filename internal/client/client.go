// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// maxResponseBodySize is the maximum response body we'll read (10 MB).
const maxResponseBodySize = 10 * 1024 * 1024

// maxRetryAfterSecs is the maximum Retry-After we'll honor (60 seconds).
const maxRetryAfterSecs = 60

// Client is the LangSmith API client.
type Client struct {
	BaseURL        string
	APIKey         string
	TenantID       string
	OrganizationID string
	UserAgent      string
	HTTPClient     *http.Client
	MaxRetries     int
}

// NewClient creates a new LangSmith API client.
func NewClient(baseURL, apiKey, tenantID, organizationID, userAgent string) *Client {
	return &Client{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		TenantID:       tenantID,
		OrganizationID: organizationID,
		UserAgent:      userAgent,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		MaxRetries: 5,
	}
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
		if c.TenantID != "" {
			req.Header.Set("X-Tenant-Id", c.TenantID)
		}
		if c.OrganizationID != "" {
			req.Header.Set("X-Organization-Id", c.OrganizationID)
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
	jitter := base * 0.2 * (2*jitterFraction() - 1) // +/- 20%
	return time.Duration(base + jitter)
}

// jitterFraction returns a value in [0, 1) for backoff randomization. It uses
// crypto/rand because retry timing can amplify observability of network behavior.
func jitterFraction() float64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Unlikely; sub-millisecond wall clock noise is sufficient for non-crypto jitter.
		return float64(time.Now().UnixNano()%1000) / 1000.0
	}
	v := binary.LittleEndian.Uint64(buf[:])
	return float64(v>>11) / (1 << 53)
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

// IsNotFound checks whether the error is a 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}
