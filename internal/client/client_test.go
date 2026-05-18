// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(srv *httptest.Server) *Client {
	c := NewClient(srv.URL, "test-key", "tenant-123", "", "test-ua/1.0")
	// Keep tests fast: cap retries and shorten the HTTP timeout.
	c.MaxRetries = 3
	c.HTTPClient.Timeout = 5 * time.Second
	return c
}

func TestAPIError_Error_WithMethodAndPath(t *testing.T) {
	e := &APIError{Method: "GET", Path: "/api/v1/info", StatusCode: 404, Body: `{"detail":"nope"}`}
	got := e.Error()
	if !strings.Contains(got, "GET") || !strings.Contains(got, "/api/v1/info") || !strings.Contains(got, "404") {
		t.Fatalf("error missing method/path/status: %q", got)
	}
}

func TestAPIError_Error_FallbackWithoutMethodPath(t *testing.T) {
	e := &APIError{StatusCode: 500, Body: "boom"}
	if got := e.Error(); !strings.Contains(got, "status 500") {
		t.Fatalf("fallback format wrong: %q", got)
	}
}

func TestIsNotFound(t *testing.T) {
	if IsNotFound(nil) {
		t.Fatal("IsNotFound(nil) should be false")
	}
	if IsNotFound(errors.New("not an APIError")) {
		t.Fatal("IsNotFound on non-APIError should be false")
	}
	if IsNotFound(&APIError{StatusCode: 500}) {
		t.Fatal("500 is not a 404")
	}
	if !IsNotFound(&APIError{StatusCode: 404}) {
		t.Fatal("404 should be recognised")
	}
}

func TestClient_SetsHeaders(t *testing.T) {
	var seenAuth, seenTenant, seenUA, seenAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("X-API-Key")
		seenTenant = r.Header.Get("X-Tenant-Id")
		seenUA = r.Header.Get("User-Agent")
		seenAccept = r.Header.Get("Accept")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if err := c.Get(context.Background(), "/anything", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if seenAuth != "test-key" {
		t.Errorf("X-API-Key = %q", seenAuth)
	}
	if seenTenant != "tenant-123" {
		t.Errorf("X-Tenant-Id = %q", seenTenant)
	}
	if seenUA != "test-ua/1.0" {
		t.Errorf("User-Agent = %q", seenUA)
	}
	if seenAccept != "application/json" {
		t.Errorf("Accept = %q", seenAccept)
	}
}

func TestClient_RetriesOn500ThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			http.Error(w, `{"detail":"flap"}`, http.StatusBadGateway)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 5
	var resp struct {
		OK bool `json:"ok"`
	}
	if err := c.Get(context.Background(), "/path", nil, &resp); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 calls (2 retries + success), got %d", got)
	}
	if !resp.OK {
		t.Fatalf("response not unmarshaled")
	}
}

func TestClient_RetriesOn408(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			http.Error(w, "timeout", http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 3
	if err := c.Get(context.Background(), "/path", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls (1 retry + success), got %d", got)
	}
}

func TestClient_429HonorsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	var firstCall, secondCall time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			firstCall = time.Now()
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		secondCall = time.Now()
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 3
	if err := c.Get(context.Background(), "/path", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
	gap := secondCall.Sub(firstCall)
	if gap < 900*time.Millisecond {
		t.Fatalf("Retry-After=1s not honored; gap was %s", gap)
	}
}

func TestClient_NonRetriable4xxReturnsImmediately(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"detail":"forbidden"}`, http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 5
	err := c.Get(context.Background(), "/path", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 {
		t.Fatalf("expected APIError 403, got %T %v", err, err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("403 should not retry; got %d calls", got)
	}
	if apiErr.Method != "GET" || apiErr.Path != "/path" {
		t.Errorf("APIError missing context: %+v", apiErr)
	}
}

func TestClient_404IsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"gone"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	err := c.Get(context.Background(), "/missing", nil, nil)
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound(err) = false; err = %v", err)
	}
}

func TestClient_ContextCancellationAborts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 10

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before first request

	err := c.Get(ctx, "/path", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestClient_DoesNotRetryNetworkErrorBeyondMax(t *testing.T) {
	// httptest server that immediately closes connections to simulate a hard
	// network failure on every attempt.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 2

	err := c.Get(context.Background(), "/path", nil, nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "after") {
		t.Fatalf("expected 'failed after N retries' wrapper, got %q", err.Error())
	}
}

func TestClient_PostMarshalsBody(t *testing.T) {
	var gotBody string
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	body := map[string]string{"name": "hello"}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.Post(context.Background(), "/things", body, &resp); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if !strings.Contains(gotBody, `"name":"hello"`) {
		t.Errorf("body = %q", gotBody)
	}
	if resp.ID != "abc" {
		t.Errorf("resp.ID = %q", resp.ID)
	}
}

func TestClient_SetsOrganizationHeader(t *testing.T) {
	var org string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		org = r.Header.Get("X-Organization-Id")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "tenant-123", "org-456", "ua")
	c.MaxRetries = 0
	c.HTTPClient.Timeout = 5 * time.Second
	if err := c.Get(context.Background(), "/x", nil, nil); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if org != "org-456" {
		t.Fatalf("X-Organization-Id = %q", org)
	}
}

func TestClient_PostWithQuery_Patch_Put_Delete_Variants(t *testing.T) {
	var lastMethod, lastPath, lastRaw string
	var lastBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path
		lastRaw = r.URL.RawQuery
		lastBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
		if strings.Contains(r.URL.Path, "/with-result") {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	ctx := context.Background()
	q := url.Values{}
	q.Set("k", "v")

	if err := c.PostWithQuery(ctx, "/postq", q, map[string]int{"n": 1}, nil); err != nil {
		t.Fatalf("PostWithQuery: %v", err)
	}
	if lastMethod != "POST" || lastPath != "/postq" || !strings.Contains(lastRaw, "k=v") {
		t.Fatalf("PostWithQuery: method=%q path=%q raw=%q", lastMethod, lastPath, lastRaw)
	}

	if err := c.Patch(ctx, "/patch", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if lastMethod != "PATCH" {
		t.Fatalf("Patch method = %q", lastMethod)
	}

	if err := c.Put(ctx, "/put", map[string]bool{"x": true}, nil); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if lastMethod != "PUT" {
		t.Fatalf("Put method = %q", lastMethod)
	}

	var delQuery url.Values
	if err := c.DeleteWithQuery(ctx, "/delq", delQuery); err != nil {
		t.Fatalf("DeleteWithQuery: %v", err)
	}
	if lastMethod != "DELETE" || lastPath != "/delq" {
		t.Fatalf("DeleteWithQuery: method=%q path=%q", lastMethod, lastPath)
	}

	if err := c.DeleteWithBody(ctx, "/delbody", map[string]string{"id": "z"}); err != nil {
		t.Fatalf("DeleteWithBody: %v", err)
	}
	if lastMethod != "DELETE" || !strings.Contains(string(lastBody), `"id":"z"`) {
		t.Fatalf("DeleteWithBody: method=%q body=%q", lastMethod, string(lastBody))
	}

	if err := c.Delete(ctx, "/del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if lastMethod != "DELETE" || lastPath != "/del" {
		t.Fatalf("Delete: method=%q path=%q", lastMethod, lastPath)
	}

	var resp struct{ OK bool `json:"ok"` }
	if err := c.PostWithQuery(ctx, "/with-result", q, nil, &resp); err != nil {
		t.Fatalf("PostWithQuery result: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected JSON result")
	}
}

func TestClient_PutWithQueryEncodesQuery(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	q := make(map[string][]string)
	q["share_projects"] = []string{"true"}
	if err := c.PutWithQuery(context.Background(), "/share", q, nil, nil); err != nil {
		t.Fatalf("PutWithQuery: %v", err)
	}
	if !strings.Contains(gotURL, "share_projects=true") {
		t.Errorf("query not encoded: %q", gotURL)
	}
}

func TestRetryDelay_GrowsExponentially(t *testing.T) {
	d1 := retryDelay(1)
	d2 := retryDelay(2)
	d3 := retryDelay(3)
	// Allow for ±20% jitter; assert each attempt is meaningfully bigger.
	if d2 < d1 || d3 < d2 {
		t.Fatalf("delays not monotonically growing: %s %s %s", d1, d2, d3)
	}
}
