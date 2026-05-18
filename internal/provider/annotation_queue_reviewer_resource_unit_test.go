// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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

func TestAnnotationQueueReviewerResource_framework(t *testing.T) {
	const queueID = "cccccccc-0000-4000-8000-000000000001"
	const identityID = "dddddddd-0000-4000-8000-000000000002"

	postPath := "/v1/platform/annotation-queues/" + queueID + "/reviewers"
	deletePath := postPath + "/" + identityID

	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodPost && r.URL.Path == postPath:
			var body addReviewerRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.IdentityID != identityID {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"identity_id": identityID})

		case r.Method == http.MethodDelete && r.URL.Path == deletePath:
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
					resource.TestCheckResourceAttr("langsmith_annotation_queue_reviewer.r", "id", queueID+":"+identityID),
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
