// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	clientpkg "github.com/bogware/terraform-provider-langsmith/internal/client"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAnnotationQueueReviewerResource_Configure(t *testing.T) {
	t.Parallel()
	var r AnnotationQueueReviewerResource
	var resp fwresource.ConfigureResponse

	r.Configure(context.Background(), fwresource.ConfigureRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no diagnostics for nil provider data")
	}

	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: "not-a-client"}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected error for wrong provider data type")
	}
}

func TestAnnotationQueueReviewerResource_Metadata_Schema(t *testing.T) {
	t.Parallel()
	var r AnnotationQueueReviewerResource
	var meta fwresource.MetadataResponse
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "langsmith"}, &meta)
	if want := "langsmith_annotation_queue_reviewer"; meta.TypeName != want {
		t.Fatalf("TypeName = %q, want %q", meta.TypeName, want)
	}
	var schemaResp fwresource.SchemaResponse
	r.Schema(context.Background(), fwresource.SchemaRequest{}, &schemaResp)
	if schemaResp.Schema.Attributes == nil {
		t.Fatal("expected schema attributes")
	}
}

func TestAnnotationQueueReviewerAPIPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		queueID    string
		identityID string
		want       string
	}{
		{"q1", "", "/v1/platform/annotation-queues/q1/reviewers"},
		{"q1", "i1", "/v1/platform/annotation-queues/q1/reviewers/i1"},
		{"a/b", "c/d", "/v1/platform/annotation-queues/a%2Fb/reviewers/c%2Fd"},
	}
	for _, tc := range cases {
		if got := annotationQueueReviewerAPIPath(tc.queueID, tc.identityID); got != tc.want {
			t.Errorf("annotationQueueReviewerAPIPath(%q, %q) = %q, want %q", tc.queueID, tc.identityID, got, tc.want)
		}
	}
}

// TestAnnotationQueueReviewerClientCRUD exercises the reviewer HTTP paths (POST/GET/DELETE
// and 404 drift) via a mock HTTP transport, without requiring a live API.
func TestAnnotationQueueReviewerClientCRUD(t *testing.T) {
	const queueID = "aaaaaaaa-0000-4000-8000-000000000001"
	const identityID = "bbbbbbbb-0000-4000-8000-000000000002"

	postPath := "/v1/platform/annotation-queues/" + queueID + "/reviewers"
	idPath := postPath + "/" + identityID

	reviewerPresent := false

	rt := &mockRoundTripper{fn: func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == postPath:
			body, _ := io.ReadAll(req.Body)
			var cr annotationQueueReviewerAddRequest
			_ = json.Unmarshal(body, &cr)
			reviewerPresent = true
			out := annotationQueueReviewerAPIResponse(cr)
			b, _ := json.Marshal(out)
			return jsonResp(b)

		case req.Method == http.MethodGet && req.URL.Path == idPath:
			if !reviewerPresent {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader(`{"detail":"not found"}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}
			}
			out := annotationQueueReviewerAPIResponse{IdentityID: identityID}
			b, _ := json.Marshal(out)
			return jsonResp(b)

		case req.Method == http.MethodDelete && req.URL.Path == idPath:
			reviewerPresent = false
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}
		}
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(fmt.Sprintf("unexpected %s %s", req.Method, req.URL.Path))),
		}
	}}

	c := clientpkg.NewClient("http://example", "key", "", "", "ua")
	c.HTTPClient.Transport = rt
	c.MaxRetries = 0
	ctx := context.Background()

	// Create
	var created annotationQueueReviewerAPIResponse
	if err := c.Post(ctx, annotationQueueReviewerAPIPath(queueID, ""), annotationQueueReviewerAddRequest{IdentityID: identityID}, &created); err != nil {
		t.Fatal(err)
	}
	if created.IdentityID != identityID {
		t.Fatalf("created identity_id = %q, want %q", created.IdentityID, identityID)
	}

	// Read
	var got annotationQueueReviewerAPIResponse
	if err := c.Get(ctx, annotationQueueReviewerAPIPath(queueID, identityID), nil, &got); err != nil {
		t.Fatal(err)
	}
	if got.IdentityID != identityID {
		t.Fatalf("read identity_id = %q, want %q", got.IdentityID, identityID)
	}

	// Delete
	if err := c.Delete(ctx, annotationQueueReviewerAPIPath(queueID, identityID)); err != nil {
		t.Fatal(err)
	}

	// Read after delete — must return 404 (drift detection)
	if err := c.Get(ctx, annotationQueueReviewerAPIPath(queueID, identityID), nil, &got); !clientpkg.IsNotFound(err) {
		t.Fatalf("expected IsNotFound after delete, got %v", err)
	}
}

// TestAnnotationQueueReviewerResource_framework exercises the reviewer resource
// through the full Terraform provider framework against a local HTTP test server.
// It covers Create, Read, and ImportState. HTTP-level 404 handling after delete is
// covered by TestAnnotationQueueReviewerClientCRUD (Read removes state on 404,
// which does not reliably surface as ExpectNonEmptyPlan across Terraform versions).
func TestAnnotationQueueReviewerResource_framework(t *testing.T) {
	const queueID = "cccccccc-0000-4000-8000-000000000001"
	const identityID = "dddddddd-0000-4000-8000-000000000002"

	postPath := "/v1/platform/annotation-queues/" + queueID + "/reviewers"
	idPath := postPath + "/" + identityID

	var mu sync.Mutex
	reviewers := map[string]bool{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodPost && r.URL.Path == postPath:
			var body annotationQueueReviewerAddRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			reviewers[body.IdentityID] = true
			resp := annotationQueueReviewerAPIResponse(body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodGet && r.URL.Path == idPath:
			if !reviewers[identityID] {
				http.NotFound(w, r)
				return
			}
			resp := annotationQueueReviewerAPIResponse{IdentityID: identityID}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodDelete && r.URL.Path == idPath:
			delete(reviewers, identityID)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := fmt.Sprintf(`
resource "langsmith_annotation_queue_reviewer" "r" {
  queue_id    = %[1]q
  identity_id = %[2]q
}
`, queueID, identityID)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_annotation_queue_reviewer.r", "queue_id", queueID),
					resource.TestCheckResourceAttr("langsmith_annotation_queue_reviewer.r", "identity_id", identityID),
					resource.TestCheckResourceAttr("langsmith_annotation_queue_reviewer.r", "id", queueID+"/"+identityID),
				),
			},
			{
				ResourceName:      "langsmith_annotation_queue_reviewer.r",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
