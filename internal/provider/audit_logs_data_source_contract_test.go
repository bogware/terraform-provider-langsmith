// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAuditLogsDataSource_contract(t *testing.T) {
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
				http.Error(w, "unexpected operations: "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			if q.Get("cursor") != "opaque-page-2" {
				http.Error(w, "unexpected cursor", http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(r.Body)
			if len(body) > 0 {
				http.Error(w, "unexpected request body", http.StatusBadRequest)
				return
			}
			nextCur := "next-cursor-token"
			resp := listAuditLogsAPIResponse{
				Cursor: &nextCur,
				Items: []json.RawMessage{
					json.RawMessage(`{"class_uid":6003,"activity_name":"unit-test"}`),
				},
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

	cfg := `data "langsmith_audit_logs" "a" {
  start_time   = "2026-01-01T00:00:00Z"
  end_time     = "2026-01-02T00:00:00Z"
  workspace_id = "990e8400-e29b-41d4-a716-446655440099"
  operations   = ["create_api_key", "delete_api_key"]
  limit        = 25
  cursor       = "opaque-page-2"
}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_audit_logs.a", "items.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_audit_logs.a", "next_cursor", "next-cursor-token"),
					resource.TestCheckResourceAttrSet("data.langsmith_audit_logs.a", "id"),
				),
			},
		},
	})
}
