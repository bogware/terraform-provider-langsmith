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

func TestAccDataPlanesDataSource_contract(t *testing.T) {
	payload := dataPlanesAPIResponse{
		DataPlanes: []dataPlaneAPI{
			{
				ID:         "dp-1",
				Name:       "Plane A",
				APIURL:     "https://dp.example",
				Region:     "us-east-1",
				Status:     json.RawMessage(`{"ready":true}`),
				Workspaces: json.RawMessage(`["w1"]`),
				CreatedAt:  "2026-01-01T00:00:00Z",
			},
		},
	}
	raw, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/orgs/current/data-planes":
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
				Config: `data "langsmith_data_planes" "d" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.0.id", "dp-1"),
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.0.name", "Plane A"),
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.0.api_url", "https://dp.example"),
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.0.region", "us-east-1"),
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.0.status", `{"ready":true}`),
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.0.workspaces", `["w1"]`),
					resource.TestCheckResourceAttr("data.langsmith_data_planes.d", "data_planes.0.created_at", "2026-01-01T00:00:00Z"),
				),
			},
		},
	})
}
