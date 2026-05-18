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

func TestToolResource_framework(t *testing.T) {
	const handle = "tf-tool-contract"
	toolID := "tool-uuid-1111-4111-8111-111111111111"

	var mu sync.Mutex
	stored := map[string]*toolAPI{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathTool := "/v1/platform/tools/" + handle
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/v1/platform/tools":
			var body toolCreate
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			rec := &toolAPI{
				ID:          toolID,
				Handle:      body.Handle,
				Name:        body.Name,
				Description: body.Description,
				Parameters:  body.Parameters,
				Returns:     map[string]interface{}{},
				Metadata:    map[string]interface{}{},
				Enabled:     true,
				TenantID:    "tenant-1",
				CreatedAt:   "2026-01-01T12:00:00Z",
				UpdatedAt:   "2026-01-01T12:00:00Z",
			}
			if body.Returns != nil {
				rec.Returns = body.Returns
			}
			if body.Metadata != nil {
				rec.Metadata = body.Metadata
			}
			if body.Enabled != nil {
				rec.Enabled = *body.Enabled
			}
			mu.Lock()
			stored[handle] = rec
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(rec)
			return

		case r.Method == http.MethodGet && r.URL.Path == pathTool:
			mu.Lock()
			cur := stored[handle]
			mu.Unlock()
			if cur == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cur)
			return

		case r.Method == http.MethodPatch && r.URL.Path == pathTool:
			body, _ := io.ReadAll(r.Body)
			var patch toolUpdate
			_ = json.Unmarshal(body, &patch)
			mu.Lock()
			cur := stored[handle]
			mu.Unlock()
			if cur == nil {
				http.NotFound(w, r)
				return
			}
			if patch.Name != nil {
				cur.Name = *patch.Name
			}
			if patch.Description != nil {
				cur.Description = *patch.Description
			}
			if patch.Parameters != nil {
				cur.Parameters = patch.Parameters
			}
			if patch.Enabled != nil {
				cur.Enabled = *patch.Enabled
			}
			cur.UpdatedAt = "2026-02-02T12:00:00Z"
			mu.Lock()
			stored[handle] = cur
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cur)
			return

		case r.Method == http.MethodDelete && r.URL.Path == pathTool:
			mu.Lock()
			delete(stored, handle)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return

		default:
			http.Error(w, fmt.Sprintf("unexpected %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfgCreate := `
resource "langsmith_tool" "t" {
  handle      = "` + handle + `"
  name        = "Contract Tool"
  description = "contract description"
  parameters  = jsonencode({ "type" = "object", "properties" = {} })
  returns     = jsonencode({ "type" = "string" })
  metadata    = jsonencode({ "tier" = "test" })
  enabled     = true
}
`
	cfgUpdate := `
resource "langsmith_tool" "t" {
  handle      = "` + handle + `"
  name        = "Contract Tool Renamed"
  description = "contract description"
  parameters  = jsonencode({ "type" = "object", "properties" = {} })
  returns     = jsonencode({ "type" = "string" })
  metadata    = jsonencode({ "tier" = "test" })
  enabled     = false
}
`

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: cfgCreate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_tool.t", "id", toolID),
					resource.TestCheckResourceAttr("langsmith_tool.t", "handle", handle),
					resource.TestCheckResourceAttr("langsmith_tool.t", "name", "Contract Tool"),
					resource.TestCheckResourceAttr("langsmith_tool.t", "tenant_id", "tenant-1"),
					resource.TestCheckResourceAttr("langsmith_tool.t", "enabled", "true"),
				),
			},
			{
				Config: cfgUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_tool.t", "name", "Contract Tool Renamed"),
					resource.TestCheckResourceAttr("langsmith_tool.t", "enabled", "false"),
					resource.TestCheckResourceAttr("langsmith_tool.t", "updated_at", "2026-02-02T12:00:00Z"),
				),
			},
			{
				ResourceName:      "langsmith_tool.t",
				ImportState:       true,
				ImportStateId:     handle,
				ImportStateVerify: true,
			},
		},
	})
}
