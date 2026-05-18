// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestFleetMCPServerResource_framework exercises langsmith_fleet_mcp_server
// against a local HTTP server that models the OpenAPI contract for
// /v1/platform/fleet/mcp-servers.
func TestFleetMCPServerResource_framework(t *testing.T) {
	const serverID = "22222222-2222-4222-8222-222222222222"

	var mu sync.Mutex
	var stored *mcpServerAPI

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/v1/platform/fleet/mcp-servers":
			var body createMcpServerPayload
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			rec := mcpServerAPI{
				ID:               serverID,
				Name:             body.Name,
				URL:              body.URL,
				AuthType:         body.AuthType,
				VendorID:         body.VendorID,
				ExternalSystemID: body.ExternalSystemID,
				OAuthMode:        body.OAuthMode,
				OAuthProviderID:  body.OAuthProviderID,
				Headers:          body.Headers,
				CanInvoke:        ptrBool(true),
				TenantID:         ptr("tenant-1"),
				CreatedAt:        ptr("2026-03-01T12:00:00Z"),
				UpdatedAt:        ptr("2026-03-01T12:00:00Z"),
			}
			if body.VendorID != nil {
				mv := "mcpv-" + *body.VendorID
				rec.MCPVendorID = &mv
			}
			mu.Lock()
			stored = &rec
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(rec)
			return

		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/fleet/mcp-servers/"+serverID:
			mu.Lock()
			cur := stored
			mu.Unlock()
			if cur == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cur)
			return

		case r.Method == http.MethodPatch && r.URL.Path == "/v1/platform/fleet/mcp-servers/"+serverID:
			var patch map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			if stored != nil {
				if u, ok := patch["url"].(string); ok {
					stored.URL = u
				}
				if v, ok := patch["auth_type"].(string); ok {
					stored.AuthType = &v
				}
				if v, ok := patch["oauth_provider_id"].(string); ok {
					stored.OAuthProviderID = &v
				}
				if raw, ok := patch["headers"]; ok {
					b, _ := json.Marshal(raw)
					stored.Headers = json.RawMessage(b)
				}
				stored.UpdatedAt = ptr("2026-03-02T12:00:00Z")
			}
			cur := stored
			mu.Unlock()
			if cur == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cur)
			return

		case r.Method == http.MethodDelete && r.URL.Path == "/v1/platform/fleet/mcp-servers/"+serverID:
			mu.Lock()
			stored = nil
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return

		default:
			http.Error(w, fmt.Sprintf("not found: %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfgCreate := `
resource "langsmith_fleet_mcp_server" "test" {
  name      = "contract-mcp"
  url       = "https://mcp.example.com/v1"
  auth_type = "headers"
  headers   = "[{\"X-Test\":\"alpha\"}]"
}
`

	cfgUpdate := `
resource "langsmith_fleet_mcp_server" "test" {
  name      = "contract-mcp"
  url       = "https://mcp.example.com/v2"
  auth_type = "headers"
  headers   = "[{\"X-Test\":\"beta\"}]"
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
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "id", serverID),
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "name", "contract-mcp"),
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "url", "https://mcp.example.com/v1"),
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "auth_type", "headers"),
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "can_invoke", "true"),
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "tenant_id", "tenant-1"),
				),
			},
			{
				Config: cfgUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "url", "https://mcp.example.com/v2"),
					resource.TestCheckResourceAttr("langsmith_fleet_mcp_server.test", "updated_at", "2026-03-02T12:00:00Z"),
				),
			},
			{
				ResourceName:      "langsmith_fleet_mcp_server.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func ptrBool(b bool) *bool {
	return &b
}

func TestBuildUpdateMcpServerPayload_sparse(t *testing.T) {
	t.Parallel()

	t.Run("only changed fields are sent", func(t *testing.T) {
		t.Parallel()

		state := &FleetMCPServerResourceModel{
			URL:             types.StringValue("https://mcp.example.com/v1"),
			AuthType:        types.StringValue("headers"),
			OAuthProviderID: types.StringNull(),
			Headers:         types.StringValue(`[{"X-Test":"alpha"}]`),
		}
		plan := &FleetMCPServerResourceModel{
			URL:             types.StringValue("https://mcp.example.com/v2"),
			AuthType:        types.StringValue("headers"),
			OAuthProviderID: types.StringNull(),
			Headers:         types.StringValue(`[{"X-Test":"beta"}]`),
		}

		patch, diags := buildUpdateMcpServerPayload(plan, state)
		if diags.HasError() {
			t.Fatalf("expected no diagnostics, got: %#v", diags)
		}

		if len(patch) != 2 {
			t.Fatalf("expected sparse patch with 2 fields, got %d: %#v", len(patch), patch)
		}
		if got := patch["url"]; got != "https://mcp.example.com/v2" {
			t.Fatalf("expected changed url in patch, got %#v", got)
		}
		if got, ok := patch["headers"].(json.RawMessage); !ok || string(got) != `[{"X-Test":"beta"}]` {
			t.Fatalf("expected changed headers in patch, got %#v", patch["headers"])
		}
	})

	t.Run("clearing optional fields sends null", func(t *testing.T) {
		t.Parallel()

		state := &FleetMCPServerResourceModel{
			URL:             types.StringValue("https://mcp.example.com/v1"),
			AuthType:        types.StringValue("oauth"),
			OAuthProviderID: types.StringValue("provider-123"),
			Headers:         types.StringValue(`[{"X-Test":"alpha"}]`),
		}
		plan := &FleetMCPServerResourceModel{
			URL:             types.StringValue("https://mcp.example.com/v1"),
			AuthType:        types.StringNull(),
			OAuthProviderID: types.StringNull(),
			Headers:         types.StringNull(),
		}

		patch, diags := buildUpdateMcpServerPayload(plan, state)
		if diags.HasError() {
			t.Fatalf("expected no diagnostics, got: %#v", diags)
		}

		if got, ok := patch["auth_type"]; !ok || got != nil {
			t.Fatalf("expected auth_type to be cleared, got %#v", got)
		}
		if got, ok := patch["oauth_provider_id"]; !ok || got != nil {
			t.Fatalf("expected oauth_provider_id to be cleared, got %#v", got)
		}
		if got, ok := patch["headers"]; !ok || got != nil {
			t.Fatalf("expected headers to be cleared, got %#v", got)
		}
		if _, ok := patch["url"]; ok {
			t.Fatalf("expected unchanged url to be omitted, got %#v", patch["url"])
		}
	})
}

func TestMapMcpServerAPIToModel_preservesEquivalentHeadersString(t *testing.T) {
	t.Parallel()

	data := &FleetMCPServerResourceModel{
		Headers: types.StringValue(`[{"Z":"1","A":"2"}]`),
	}

	mapMcpServerAPIToModel(&mcpServerAPI{
		ID:      "server-1",
		Name:    "contract-mcp",
		URL:     "https://mcp.example.com/v1",
		Headers: json.RawMessage(`[{"A":"2","Z":"1"}]`),
	}, data)

	if got := data.Headers.ValueString(); got != `[{"Z":"1","A":"2"}]` {
		t.Fatalf("expected existing header formatting to be preserved, got %q", got)
	}
}

func TestValidateFleetMCPServerConfig(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		data      FleetMCPServerResourceModel
		wantError bool
	}{
		"oauth requires oauth_provider_id": {
			data: FleetMCPServerResourceModel{
				AuthType: types.StringValue("oauth"),
			},
			wantError: true,
		},
		"headers requires headers payload": {
			data: FleetMCPServerResourceModel{
				AuthType: types.StringValue("headers"),
			},
			wantError: true,
		},
		"headers auth_type rejects oauth fields": {
			data: FleetMCPServerResourceModel{
				AuthType:        types.StringValue("headers"),
				Headers:         types.StringValue(`[{"X-Test":"alpha"}]`),
				OAuthProviderID: types.StringValue("provider-123"),
				OAuthMode:       types.StringValue("legacy_shared_provider"),
			},
			wantError: true,
		},
		"oauth auth_type rejects headers": {
			data: FleetMCPServerResourceModel{
				AuthType:        types.StringValue("oauth"),
				OAuthProviderID: types.StringValue("provider-123"),
				Headers:         types.StringValue(`[{"X-Test":"alpha"}]`),
			},
			wantError: true,
		},
		"dependent fields require auth_type": {
			data: FleetMCPServerResourceModel{
				Headers: types.StringValue(`[{"X-Test":"alpha"}]`),
			},
			wantError: true,
		},
		"valid headers configuration": {
			data: FleetMCPServerResourceModel{
				AuthType: types.StringValue("headers"),
				Headers:  types.StringValue(`[{"X-Test":"alpha"}]`),
			},
		},
		"valid oauth configuration": {
			data: FleetMCPServerResourceModel{
				AuthType:        types.StringValue("oauth"),
				OAuthProviderID: types.StringValue("provider-123"),
				OAuthMode:       types.StringValue("legacy_shared_provider"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			diags := validateFleetMCPServerConfig(&tc.data)
			if diags.HasError() != tc.wantError {
				t.Fatalf("expected wantError=%t, got diagnostics=%#v", tc.wantError, diags)
			}
		})
	}
}
