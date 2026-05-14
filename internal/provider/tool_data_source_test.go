// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func toolTestAPIPayload() map[string]interface{} {
	return map[string]interface{}{
		"id":          "550e8400-e29b-41d4-a716-446655440000",
		"handle":      "demo-tool",
		"name":        "Demo Tool",
		"description": "A demo platform tool",
		"enabled":     true,
		"tenant_id":   "tenant-1",
		"created_at":  "2025-01-01T00:00:00Z",
		"updated_at":  "2025-01-02T00:00:00Z",
		"metadata":    map[string]interface{}{"k": "v"},
		"parameters":  map[string]interface{}{"type": "object"},
		"returns":     map[string]interface{}{"type": "string"},
	}
}

func newToolMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/tools/demo-tool":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(toolTestAPIPayload())
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/tools/id/550e8400-e29b-41d4-a716-446655440000":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(toolTestAPIPayload())
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}))
}

func TestAccToolDataSource_byHandle(t *testing.T) {
	srv := newToolMockServer(t)
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_tool" "test" { handle = "demo-tool" }`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "id", "550e8400-e29b-41d4-a716-446655440000"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "handle", "demo-tool"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "name", "Demo Tool"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "description", "A demo platform tool"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "enabled", "true"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "tenant_id", "tenant-1"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "created_at", "2025-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "updated_at", "2025-01-02T00:00:00Z"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "metadata", `{"k":"v"}`),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "parameters", `{"type":"object"}`),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "returns", `{"type":"string"}`),
				),
			},
		},
	})
}

func TestAccToolDataSource_byID(t *testing.T) {
	srv := newToolMockServer(t)
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_tool" "test" { id = "550e8400-e29b-41d4-a716-446655440000" }`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "handle", "demo-tool"),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "name", "Demo Tool"),
				),
			},
		},
	})
}

func TestAccToolDataSource_validateNeither(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/info" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_tool" "test" {}`,
				// Match provider diagnostic summary (see tag_key_data_source) and detail line.
				ExpectError: regexp.MustCompile(`(?s)Missing Required Attribute.*Either "handle" or "id" must be specified`),
			},
		},
	})
}

func TestAccToolDataSource_validateBoth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/info" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "langsmith_tool" "test" {
  handle = "demo-tool"
  id     = "550e8400-e29b-41d4-a716-446655440000"
}
`,
				ExpectError: regexp.MustCompile(`(?s)Conflicting Arguments.*mutually\s+exclusive`),
			},
		},
	})
}
