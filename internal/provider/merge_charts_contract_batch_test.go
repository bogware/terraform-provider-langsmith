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

var previewSeriesWorkspace = `[{"id":"550e8400-e29b-41d4-a716-446655440001","filters":{"session":["proj-1"]}}]`
var previewSeriesOrg = `[{"id":"660e8400-e29b-41d4-a716-446655440002"}]`

func TestAccChartPreviewDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/charts/preview":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"series_id":"550e8400-e29b-41d4-a716-446655440001","timestamp":"2026-01-01T00:00:00Z","value":1}]}`))
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
		Steps: []resource.TestStep{{
			Config: `
data "langsmith_chart_preview" "p" {
  series = ` + fmt.Sprintf("%q", previewSeriesWorkspace) + `
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
}
`,
			Check: resource.TestCheckResourceAttr("data.langsmith_chart_preview.p", "data",
				`[{"series_id":"550e8400-e29b-41d4-a716-446655440001","timestamp":"2026-01-01T00:00:00Z","value":1}]`),
		}},
	})
}

func TestAccOrgChartPreviewDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/org-charts/preview":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
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
		Steps: []resource.TestStep{{
			Config: `
data "langsmith_org_chart_preview" "p" {
  series = ` + fmt.Sprintf("%q", previewSeriesOrg) + `
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.langsmith_org_chart_preview.p", "data", `[]`),
			),
		}},
	})
}

func TestAccChartSectionDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/charts/section":
			idx := int64(1)
			c := int64(2)
			ca := "2026-01-01T00:00:00Z"
			ma := "2026-01-02T00:00:00Z"
			desc := "sec"
			list := []chartSectionListAPIResponse{{
				ID: "sec-ws-1", Title: "Alpha", Description: &desc,
				Index: &idx, ChartCount: &c, CreatedAt: &ca, ModifiedAt: &ma,
			}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
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
		Steps: []resource.TestStep{{
			Config: `data "langsmith_chart_section" "s" { id = "sec-ws-1" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.langsmith_chart_section.s", "title", "Alpha"),
				resource.TestCheckResourceAttr("data.langsmith_chart_section.s", "chart_count", "2"),
			),
		}},
	})
}

func TestAccOrgChartSectionDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/org-charts/section":
			list := []chartSectionListAPIResponse{{ID: "sec-o-1", Title: "Beta"}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
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
		Steps: []resource.TestStep{{
			Config: `data "langsmith_org_chart_section" "s" { title = "Beta" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.langsmith_org_chart_section.s", "id", "sec-o-1"),
			),
		}},
	})
}

func TestAccChartResource_workspace_contract(t *testing.T) {
	chartID := "ws-chart-6666-4666-8666-666666666666"
	currentTitle := "WS Chart v1"
	var mu sync.Mutex
	exists := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathID := "/api/v1/charts/" + chartID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/charts/create":
			mu.Lock()
			exists = true
			tit := currentTitle
			mu.Unlock()
			idx := int64(1)
			out := chartAPIResponse{
				ID: chartID, Title: tit, Index: &idx, ChartType: "line",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPost && r.URL.Path == pathID:
			mu.Lock()
			ok := exists
			tit := currentTitle
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = io.ReadAll(r.Body)
			idx := int64(1)
			out := chartAPIResponse{
				ID: chartID, Title: tit, Index: &idx, ChartType: "line",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPatch && r.URL.Path == pathID:
			mu.Lock()
			currentTitle = "WS Chart v2"
			tit := currentTitle
			mu.Unlock()
			idx := int64(1)
			out := chartAPIResponse{
				ID: chartID, Title: tit, Index: &idx, ChartType: "line",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodDelete && r.URL.Path == pathID:
			mu.Lock()
			exists = false
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_chart" "c" {
  title          = "WS Chart v1"
  chart_type     = "line"
  series         = jsonencode([])
  metadata       = jsonencode({})
  common_filters = jsonencode({})
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_chart.c", "id", chartID),
				),
			},
			{
				Config: `
resource "langsmith_chart" "c" {
  title          = "WS Chart v2"
  chart_type     = "line"
  series         = jsonencode([])
  metadata       = jsonencode({})
  common_filters = jsonencode({})
}`,
			},
			{
				ResourceName:            "langsmith_chart.c",
				ImportState:             true,
				ImportStateId:           chartID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"series", "section_id"},
			},
		},
	})
}

func TestAccChartSectionResource_workspace_contract(t *testing.T) {
	secID := "sec-res-ws-7777-4777-8777-777777777777"
	var mu sync.Mutex
	title := "Sec WS"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathID := "/api/v1/charts/section/" + secID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/charts/section":
			mu.Lock()
			tit := title
			mu.Unlock()
			idx := int64(0)
			ca := "2026-01-01T00:00:00Z"
			out := chartSectionAPIResponse{
				ID: secID, Title: tit, Index: &idx,
				CreatedAt: &ca, ModifiedAt: &ca,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPost && r.URL.Path == pathID:
			mu.Lock()
			tit := title
			mu.Unlock()
			idx := int64(0)
			out := chartSectionAPIResponse{ID: secID, Title: tit, Index: &idx}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPatch && r.URL.Path == pathID:
			mu.Lock()
			title = "Sec WS renamed"
			tit := title
			mu.Unlock()
			idx := int64(0)
			out := chartSectionAPIResponse{ID: secID, Title: tit, Index: &idx}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodDelete && r.URL.Path == pathID:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{Config: `resource   "langsmith_chart_section" "s" { title = "Sec WS" }`},
			{Config: `resource "langsmith_chart_section" "s" { title = "Sec WS renamed" }`},
		},
	})
}

func TestAccOrgChartSectionResource_contract_full(t *testing.T) {
	secID := "sec-org-8888-4888-8888-888888888888"
	var mu sync.Mutex
	title := "Org Sec"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathID := "/api/v1/org-charts/section/" + secID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/org-charts/section":
			mu.Lock()
			tit := title
			mu.Unlock()
			idx := int64(3)
			ca := "2026-02-01T00:00:00Z"
			out := chartSectionAPIResponse{
				ID: secID, Title: tit, Index: &idx,
				CreatedAt: &ca, ModifiedAt: &ca,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPost && r.URL.Path == pathID:
			mu.Lock()
			tit := title
			mu.Unlock()
			idx := int64(3)
			out := chartSectionAPIResponse{ID: secID, Title: tit, Index: &idx}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPatch && r.URL.Path == pathID:
			mu.Lock()
			title = "Org Sec New"
			tit := title
			mu.Unlock()
			idx := int64(3)
			out := chartSectionAPIResponse{ID: secID, Title: tit, Index: &idx}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodDelete && r.URL.Path == pathID:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{Config: `resource "langsmith_org_chart_section" "s" { title = "Org Sec" }`},
			{Config: `resource "langsmith_org_chart_section" "s" { title = "Org Sec New" }`},
			{
				ResourceName:            "langsmith_org_chart_section.s",
				ImportState:             true,
				ImportStateId:           secID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"created_at", "updated_at"},
			},
		},
	})
}

func TestAccChartSectionCloneResource_contract(t *testing.T) {
	clonedID := "clone-9999-4999-8999-999999999999"
	var mu sync.Mutex
	tit := "Cloned Title"
	idx := int64(0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathID := "/api/v1/charts/section/" + clonedID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/charts/section/clone":
			mu.Lock()
			ct := tit
			ci := idx
			mu.Unlock()
			out := chartSectionAPIResponse{
				ID: clonedID, Title: ct, Index: &ci,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPost && r.URL.Path == pathID:
			mu.Lock()
			ct := tit
			ci := idx
			mu.Unlock()
			out := chartSectionAPIResponse{ID: clonedID, Title: ct, Index: &ci}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPatch && r.URL.Path == pathID:
			mu.Lock()
			tit = "Cloned Title Patched"
			idx = 1
			ct := tit
			ci := idx
			mu.Unlock()
			out := chartSectionAPIResponse{ID: clonedID, Title: ct, Index: &ci}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodDelete && r.URL.Path == pathID:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_chart_section_clone" "c" {
  source_section_id = "src-section"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_chart_section_clone.c", "id", clonedID),
				),
			},
			{
				Config: `
resource "langsmith_chart_section_clone" "c" {
  source_section_id = "src-section"
  title             = "Cloned Title Patched"
  index             = 1
}
`,
			},
		},
	})
}
