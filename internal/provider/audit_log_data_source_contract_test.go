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

func TestAccAuditLogDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/audit-logs":
			if r.URL.Query().Get("start_time") == "" || r.URL.Query().Get("end_time") == "" {
				http.Error(w, "missing time bounds", http.StatusBadRequest)
				return
			}
			next := "cursor-next"
			payload := auditLogResponse{
				Cursor: &next,
				Items:  []json.RawMessage{json.RawMessage(`{"activity_id":"a1"}`)},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
			return

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
data "langsmith_audit_log" "x" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  limit      = 10
  operations = ["read", "write"]
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_audit_log.x", "items.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_audit_log.x", "items.0", `{"activity_id":"a1"}`),
					resource.TestCheckResourceAttr("data.langsmith_audit_log.x", "next_cursor", "cursor-next"),
				),
			},
		},
	})
}

func TestAccAuditLogDataSource_contract_queryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/audit-logs":
			q := r.URL.Query()
			if q.Get("start_time") != "2026-01-01T00:00:00Z" || q.Get("end_time") != "2026-01-02T00:00:00Z" {
				http.Error(w, "unexpected time range", http.StatusBadRequest)
				return
			}
			if q.Get("limit") != "25" {
				http.Error(w, "unexpected limit", http.StatusBadRequest)
				return
			}
			if q.Get("workspace_id") != "990e8400-e29b-41d4-a716-446655440099" {
				http.Error(w, "unexpected workspace_id", http.StatusBadRequest)
				return
			}
			ops := q["operations"]
			if len(ops) != 2 || ops[0] != "create_api_key" || ops[1] != "delete_api_key" {
				http.Error(w, "unexpected operations", http.StatusBadRequest)
				return
			}
			if q.Get("cursor") != "opaque-page-2" {
				http.Error(w, "unexpected cursor", http.StatusBadRequest)
				return
			}
			nextCur := "next-cursor-token"
			payload := auditLogResponse{
				Cursor: &nextCur,
				Items:  []json.RawMessage{json.RawMessage(`{"class_uid":6003,"activity_name":"unit-test"}`)},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
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
				Config: `data "langsmith_audit_log" "a" {
  start_time   = "2026-01-01T00:00:00Z"
  end_time     = "2026-01-02T00:00:00Z"
  workspace_id = "990e8400-e29b-41d4-a716-446655440099"
  operations   = ["create_api_key", "delete_api_key"]
  limit        = 25
  cursor       = "opaque-page-2"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_audit_log.a", "items.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_audit_log.a", "next_cursor", "next-cursor-token"),
				),
			},
		},
	})
}

func TestAccAuditLogDataSource_contract_noCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/audit-logs":
			payload := auditLogResponse{Items: []json.RawMessage{json.RawMessage(`{}`)}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
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
				Config: `
data "langsmith_audit_log" "x" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_audit_log.x", "items.#", "1"),
					resource.TestCheckNoResourceAttr("data.langsmith_audit_log.x", "next_cursor"),
				),
			},
		},
	})
}
