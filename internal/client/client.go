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

// Client is the LangSmith API client.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new LangSmith API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
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

// Get performs an HTTP GET request.
func (c *Client) Get(ctx context.Context, path string, query url.Values, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, query, nil, result)
}

// Post performs an HTTP POST request.
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, nil, body, result)
}

// PostWithQuery performs an HTTP POST request with query parameters.
func (c *Client) PostWithQuery(ctx context.Context, path string, query url.Values, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, query, body, result)
}

// Patch performs an HTTP PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPatch, path, nil, body, result)
}

// Put performs an HTTP PUT request.
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPut, path, nil, body, result)
}

// Delete performs an HTTP DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil, nil)
}

// DeleteWithQuery performs an HTTP DELETE request with query parameters.
func (c *Client) DeleteWithQuery(ctx context.Context, path string, query url.Values) error {
	return c.doRequest(ctx, http.MethodDelete, path, query, nil, nil)
}

// DeleteWithBody performs an HTTP DELETE with a request body.
func (c *Client) DeleteWithBody(ctx context.Context, path string, body interface{}) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, body, nil)
}

// APIError represents an error from the LangSmith API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("LangSmith API error (status %d): %s", e.StatusCode, e.Body)
}

// IsNotFound returns true if the error is a 404 Not Found.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 404
	}
	return false
}
