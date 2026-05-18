// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestDatasetShareResource_framework(t *testing.T) {
	const datasetID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

	var mu sync.Mutex
	shared := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/datasets/"+datasetID+"/share":
			mu.Lock()
			shared = true
			mu.Unlock()
			if r.URL.Query().Get("share_projects") != "true" {
				http.Error(w, "expected share_projects=true", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(datasetShareAPI{DatasetID: datasetID, ShareToken: "tok-abc"})
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/datasets/"+datasetID+"/share":
			mu.Lock()
			ok := shared
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(datasetShareAPI{DatasetID: datasetID, ShareToken: "tok-abc"})
			return

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/datasets/"+datasetID+"/share":
			mu.Lock()
			shared = false
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return

		default:
			body, _ := io.ReadAll(r.Body)
			http.Error(w, fmt.Sprintf("unexpected %s %s %s", r.Method, r.URL.Path, body), http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := fmt.Sprintf(`
resource "langsmith_dataset_share" "s" {
  dataset_id     = %[1]q
  share_projects = true
}
`, datasetID)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_dataset_share.s", "dataset_id", datasetID),
					resource.TestCheckResourceAttr("langsmith_dataset_share.s", "share_token", "tok-abc"),
				),
			},
		},
	})
}
