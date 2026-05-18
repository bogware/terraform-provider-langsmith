// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMCPVendorDataSource_contract(t *testing.T) {
	apiBody := mcpVendorAPI{
		VendorID:    "vid-1",
		ProviderID:  "pid-1",
		Name:        "Acme",
		Description: "ACME MCP",
		Icon:        "icon.png",
		Status:      "enabled",
		Settings:    map[string]interface{}{"region": "us"},
	}
	raw, _ := json.Marshal(apiBody)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/mcp-vendors/acme":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_mcp_vendor" "v" {
  vendor_slug = "acme"
}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor.v", "vendor_id", "vid-1"),
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor.v", "provider_id", "pid-1"),
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor.v", "name", "Acme"),
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor.v", "description", "ACME MCP"),
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor.v", "icon", "icon.png"),
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor.v", "status", "enabled"),
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor.v", "settings_json", `{"region":"us"}`),
				),
			},
		},
	})
}

func TestAccMCPVendorDataSource_contract_nullSettings(t *testing.T) {
	apiBody := mcpVendorAPI{
		VendorID: "vid-2", ProviderID: "p", Name: "Bare", Description: "d", Icon: "i", Status: "disabled",
		Settings: nil,
	}
	raw, _ := json.Marshal(apiBody)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/mcp-vendors/bare":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_mcp_vendor" "v" { vendor_slug = "bare" }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("data.langsmith_mcp_vendor.v", "settings_json"),
				),
			},
		},
	})
}
