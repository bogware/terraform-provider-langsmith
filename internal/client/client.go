// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is the LangSmith API client — the trusty horse that carries every
// request across the wire to the LangSmith frontier.
type Client struct {
	BaseURL    string
	APIKey     string
	TenantID   string
	HTTPClient *http.Client
}

// NewClient saddles up a fresh LangSmith API client with the given base URL,
// API key, and optional tenant ID.
func NewClient(baseURL, apiKey, tenantID string) *Client {
	return &Client{
		BaseURL:  baseURL,
		APIKey:   apiKey,
		TenantID: tenantID,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	reqURL := c.BaseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-API-Key", c.APIKey)
	if c.TenantID != "" {
		req.Header.Set("X-Tenant-Id", c.TenantID)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
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

// Get rides out with an HTTP GET request and brings back whatever the API has to say.
func (c *Client) Get(ctx context.Context, path string, query url.Values, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, query, nil, result)
}

// Post sends an HTTP POST request — staking a new claim on the LangSmith API.
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, nil, body, result)
}

// PostWithQuery sends an HTTP POST with query parameters riding shotgun.
func (c *Client) PostWithQuery(ctx context.Context, path string, query url.Values, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, query, body, result)
}

// Patch sends an HTTP PATCH request to mend what needs mending.
func (c *Client) Patch(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPatch, path, nil, body, result)
}

// Put sends an HTTP PUT request, replacing the whole lot in one go.
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPut, path, nil, body, result)
}

// Delete sends an HTTP DELETE request. No trial, no appeal.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil, nil)
}

// DeleteWithQuery sends an HTTP DELETE with query parameters to help identify the outlaw.
func (c *Client) DeleteWithQuery(ctx context.Context, path string, query url.Values) error {
	return c.doRequest(ctx, http.MethodDelete, path, query, nil, nil)
}

// DeleteWithBody sends an HTTP DELETE with a request body, for when you need
// to spell out exactly what you're putting down.
func (c *Client) DeleteWithBody(ctx context.Context, path string, body interface{}) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, body, nil)
}

// APIError represents trouble from the LangSmith API — the kind Doc Adams
// would shake his head at. Carries the HTTP status code and raw response body.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("LangSmith API error (status %d): %s", e.StatusCode, e.Body)
}

// IsNotFound checks whether the error is a 404 — the resource has skipped town
// and left no forwarding address.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 404
	}
	return false
}
