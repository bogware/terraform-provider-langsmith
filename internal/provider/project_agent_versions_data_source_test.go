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

func newProjectAgentVersionsMockServer(t *testing.T, sessionID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/sessions/"+sessionID+"/agent-versions":
			w.Header().Set("Content-Type", "application/json")
			payload := []map[string]interface{}{
				{
					"commit_sha":    "deadbeef",
					"first_seen_at": "2025-01-01T12:00:00Z",
				},
				{
					"commit_sha":    "cafebabe",
					"first_seen_at": "2025-01-02T00:00:00Z",
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}))
}

func TestAccProjectAgentVersionsDataSource_basic(t *testing.T) {
	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	srv := newProjectAgentVersionsMockServer(t, sessionID)
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_project_agent_versions" "test" {
  session_id = "` + sessionID + `"
}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.test", "id", sessionID),
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.test", "session_id", sessionID),
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.test", "agent_versions.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.test", "agent_versions.0.commit_sha", "deadbeef"),
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.test", "agent_versions.0.first_seen_at", "2025-01-01T12:00:00Z"),
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.test", "agent_versions.1.commit_sha", "cafebabe"),
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.test", "agent_versions.1.first_seen_at", "2025-01-02T00:00:00Z"),
				),
			},
		},
	})
}

func TestAccProjectAgentVersionsDataSource_empty(t *testing.T) {
	sessionID := "660e8400-e29b-41d4-a716-446655440001"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/sessions/"+sessionID+"/agent-versions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_project_agent_versions" "empty" {
  session_id = "` + sessionID + `"
}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_project_agent_versions.empty", "agent_versions.#", "0"),
				),
			},
		},
	})
}
