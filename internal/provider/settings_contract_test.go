// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSettingsResource_contract(t *testing.T) {
	tenantID := "550e8400-e29b-41d4-a716-446655440000"
	var postCalls atomic.Int32
	var mu sync.Mutex
	currentHandle := "before-handle"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/settings":
			mu.Lock()
			h := currentHandle
			mu.Unlock()
			resp := settingsTenantAPIResponse{
				ID:           tenantID,
				DisplayName:  "Contract WS",
				CreatedAt:    "2020-01-01T00:00:00Z",
				TenantHandle: &h,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/settings/handle":
			postCalls.Add(1)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var reqBody setTenantHandleRequest
			if err := json.Unmarshal(body, &reqBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if reqBody.TenantHandle != "after-handle" {
				http.Error(w, "unexpected tenant_handle in POST body", http.StatusBadRequest)
				return
			}
			mu.Lock()
			currentHandle = "after-handle"
			mu.Unlock()
			h := "after-handle"
			resp := settingsTenantAPIResponse{
				ID:           tenantID,
				DisplayName:  "Contract WS",
				CreatedAt:    "2020-01-01T00:00:00Z",
				TenantHandle: &h,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
resource "langsmith_settings" "s" {
  tenant_handle = "after-handle"
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_settings.s", "id", tenantID),
					resource.TestCheckResourceAttr("langsmith_settings.s", "tenant_handle", "after-handle"),
					resource.TestCheckResourceAttr("langsmith_settings.s", "display_name", "Contract WS"),
					resource.TestCheckResourceAttr("langsmith_settings.s", "created_at", "2020-01-01T00:00:00Z"),
				),
			},
			{
				ResourceName:      "langsmith_settings.s",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})

	if postCalls.Load() != 1 {
		t.Fatalf("expected exactly one POST /api/v1/settings/handle, got %d", postCalls.Load())
	}
}

func TestAccSettingsResource_contract_noPostWhenHandleMatches(t *testing.T) {
	tenantID := "660e8400-e29b-41d4-a716-446655440001"
	var postCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/settings":
			h := "already-set"
			resp := settingsTenantAPIResponse{
				ID:           tenantID,
				DisplayName:  "Same",
				CreatedAt:    "2021-06-01T12:00:00Z",
				TenantHandle: &h,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/settings/handle":
			postCalls.Add(1)
			http.Error(w, "unexpected POST", http.StatusBadRequest)
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
resource "langsmith_settings" "s" {
  tenant_handle = "already-set"
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
		},
	})

	if postCalls.Load() != 0 {
		t.Fatalf("expected zero POST /api/v1/settings/handle, got %d", postCalls.Load())
	}
}

func TestAccSettingsDataSource_contract(t *testing.T) {
	tenantID := "770e8400-e29b-41d4-a716-446655440002"
	h := "ds-handle"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/settings":
			resp := settingsTenantAPIResponse{
				ID:           tenantID,
				DisplayName:  "DS WS",
				CreatedAt:    "2019-03-15T08:30:00Z",
				TenantHandle: &h,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_settings" "s" {}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_settings.s", "id", tenantID),
					resource.TestCheckResourceAttr("data.langsmith_settings.s", "display_name", "DS WS"),
					resource.TestCheckResourceAttr("data.langsmith_settings.s", "tenant_handle", "ds-handle"),
					resource.TestCheckResourceAttr("data.langsmith_settings.s", "created_at", "2019-03-15T08:30:00Z"),
				),
			},
		},
	})
}
