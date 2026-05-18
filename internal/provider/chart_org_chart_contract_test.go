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

func TestAccChartDataSource_contract(t *testing.T) {
	const chartID = "chart-ds-1111-4111-8111-111111111111"
	desc := "DS chart"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/charts/"+chartID:
			out := chartSingleAPIResponse{
				ID: chartID, Title: "My Chart", Description: &desc,
				Index: 3, ChartType: "line",
				Series:        json.RawMessage(`[{"name":"s1"}]`),
				Metadata:      json.RawMessage(`{"k":"v"}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
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
				Config: `data "langsmith_chart" "c" { id = "` + chartID + `" }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_chart.c", "title", "My Chart"),
					resource.TestCheckResourceAttr("data.langsmith_chart.c", "description", "DS chart"),
					resource.TestCheckResourceAttr("data.langsmith_chart.c", "index", "3"),
					resource.TestCheckResourceAttr("data.langsmith_chart.c", "chart_type", "line"),
					resource.TestCheckResourceAttr("data.langsmith_chart.c", "series", `[{"name":"s1"}]`),
					resource.TestCheckResourceAttr("data.langsmith_chart.c", "metadata", `{"k":"v"}`),
					resource.TestCheckResourceAttr("data.langsmith_chart.c", "common_filters", `{}`),
				),
			},
		},
	})
}

func TestAccOrgChartDataSource_contract(t *testing.T) {
	const chartID = "org-ds-2222-4222-8222-222222222222"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/org-charts/"+chartID:
			out := chartSingleAPIResponse{
				ID: chartID, Title: "Org DS", Description: nil,
				Index: 0, ChartType: "barSeriesShouldNotMatter",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`null`),
				CommonFilters: json.RawMessage(`[]`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
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
				Config: `data "langsmith_org_chart" "c" { id = "` + chartID + `" }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_org_chart.c", "title", "Org DS"),
					resource.TestCheckResourceAttr("data.langsmith_org_chart.c", "chart_type", "barSeriesShouldNotMatter"),
					resource.TestCheckNoResourceAttr("data.langsmith_org_chart.c", "description"),
				),
			},
		},
	})
}

func TestAccOrgChartResource_contract(t *testing.T) {
	chartID := "org-res-3333-4333-8333-333333333333"
	var mu sync.Mutex
	exists := false
	currentTitle := "Created Org Chart"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathID := "/api/v1/org-charts/" + chartID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/org-charts/create":
			mu.Lock()
			exists = true
			mu.Unlock()
			idx := int64(5)
			mu.Lock()
			tit := currentTitle
			mu.Unlock()
			out := chartAPIResponse{
				ID: chartID, Title: tit, Description: nil, Index: &idx,
				ChartType: "line", Series: json.RawMessage(`[]`),
				Metadata: json.RawMessage(`{}`), CommonFilters: json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
			return

		case r.Method == http.MethodPost && r.URL.Path == pathID:
			mu.Lock()
			ok := exists
			tit := currentTitle
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			body, _ := io.ReadAll(r.Body)
			if len(body) < 10 || string(body) == "" {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			idx := int64(5)
			out := chartAPIResponse{
				ID: chartID, Title: tit, Index: &idx, ChartType: "line",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return

		case r.Method == http.MethodPatch && r.URL.Path == pathID:
			mu.Lock()
			currentTitle = "Updated Title"
			tit := currentTitle
			mu.Unlock()
			idx := int64(5)
			out := chartAPIResponse{
				ID: chartID, Title: tit, Index: &idx, ChartType: "line",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return

		case r.Method == http.MethodDelete && r.URL.Path == pathID:
			mu.Lock()
			exists = false
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
resource "langsmith_org_chart" "c" {
  title          = "Created Org Chart"
  chart_type     = "line"
  series         = jsonencode([])
  metadata       = jsonencode({})
  common_filters = jsonencode({})
}
`
	cfgUpdate := `
resource "langsmith_org_chart" "c" {
  title          = "Updated Title"
  chart_type     = "line"
  series         = jsonencode([])
  metadata       = jsonencode({})
  common_filters = jsonencode({})
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
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "id", chartID),
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "title", "Created Org Chart"),
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "chart_type", "line"),
				),
			},
			{
				Config: cfgUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "title", "Updated Title"),
				),
			},
			{
				ResourceName:            "langsmith_org_chart.c",
				ImportState:             true,
				ImportStateId:           chartID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"series", "section_id"},
			},
		},
	})
}
