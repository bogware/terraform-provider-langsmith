// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	t.Parallel()
	c := NewClient("https://api.example", "key", "tenant", "ua/1")
	if c.BaseURL != "https://api.example" || c.APIKey != "key" || c.TenantID != "tenant" || c.UserAgent != "ua/1" {
		t.Fatalf("unexpected fields: %#v", c)
	}
	if c.HTTPClient == nil || c.MaxRetries != 5 {
		t.Fatalf("unexpected defaults: %#v", c)
	}
}

func TestRetryDelay_positive(t *testing.T) {
	t.Parallel()
	for attempt := 1; attempt <= 6; attempt++ {
		d := retryDelay(attempt)
		if d <= 0 {
			t.Fatalf("attempt %d: non-positive duration %v", attempt, d)
		}
	}
}

func TestAPIError_Error(t *testing.T) {
	t.Parallel()
	e := &APIError{StatusCode: 418, Body: "short and stout"}
	if got := e.Error(); got == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()
	if IsNotFound(errors.New("plain")) {
		t.Fatal("expected false for plain error")
	}
	if IsNotFound(&APIError{StatusCode: 500}) {
		t.Fatal("expected false for 500")
	}
	if !IsNotFound(&APIError{StatusCode: 404}) {
		t.Fatal("expected true for 404")
	}
}

func TestClient_Get_JSON(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("X-Tenant-Id"); got != "tid" {
			http.Error(w, "missing tenant", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("User-Agent"); got != "test-ua" {
			http.Error(w, "missing ua", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hello":true}`)
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "secret", "tid", "test-ua")
	c.HTTPClient = ts.Client()

	var out map[string]bool
	if err := c.Get(context.Background(), "/", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !out["hello"] {
		t.Fatalf("unexpected decode: %#v", out)
	}
}

func TestClient_Get_APIError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTeapot)
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "k", "", "")
	c.HTTPClient = ts.Client()

	err := c.Get(context.Background(), "/x", nil, nil)
	var api *APIError
	if !errors.As(err, &api) || api.StatusCode != http.StatusTeapot {
		t.Fatalf("got err=%v", err)
	}
}

func TestClient_Delete_noContent(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)

	c := NewClient(ts.URL, "k", "", "")
	c.HTTPClient = ts.Client()

	if err := c.Delete(context.Background(), "/gone"); err != nil {
		t.Fatal(err)
	}
}
