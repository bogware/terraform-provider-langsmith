// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	clientpkg "github.com/bogware/terraform-provider-langsmith/internal/client"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestMapOrgChartResponseToState_variants(t *testing.T) {
	t.Parallel()
	idx := int64(3)
	desc := "d"
	sid := "sec-1"
	cases := []struct {
		name   string
		input  orgChartAPIResponse
		assert func(t *testing.T, m *OrgChartResourceModel)
	}{
		{
			name: "full",
			input: orgChartAPIResponse{
				ID: "c1", Title: "T", ChartType: "line",
				Description: &desc, Index: &idx, Series: json.RawMessage(`[]`),
				SectionID: &sid, Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			},
			assert: func(t *testing.T, m *OrgChartResourceModel) {
				t.Helper()
				if m.ID.ValueString() != "c1" || m.Title.ValueString() != "T" {
					t.Fatalf("id/title mismatch")
				}
				if m.Index.ValueInt64() != 3 {
					t.Fatalf("index")
				}
				if m.SectionID.ValueString() != "sec-1" {
					t.Fatalf("section_id")
				}
			},
		},
		{
			name: "nil_index",
			input: orgChartAPIResponse{
				ID: "c2", Title: "T2", ChartType: "bar",
				Series: json.RawMessage(`[1]`), Metadata: json.RawMessage(`null`),
				CommonFilters: json.RawMessage(`null`),
			},
			assert: func(t *testing.T, m *OrgChartResourceModel) {
				t.Helper()
				if !m.Index.IsNull() {
					t.Fatalf("index should be null")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m OrgChartResourceModel
			mapOrgChartResponseToState(&m, &tc.input)
			tc.assert(t, &m)
		})
	}
}

func TestMapOrgChartSectionResponseToState_variants(t *testing.T) {
	t.Parallel()
	idx := int64(2)
	ca := "2020-01-01T00:00:00Z"
	ma := "2020-01-02T00:00:00Z"
	desc := "x"
	cases := []struct {
		name   string
		input  orgChartSectionAPIResponse
		assert func(t *testing.T, m *OrgChartSectionResourceModel)
	}{
		{
			name: "with_timestamps",
			input: orgChartSectionAPIResponse{
				ID: "s1", Title: "Sec", Description: &desc, Index: &idx,
				CreatedAt: &ca, ModifiedAt: &ma,
			},
			assert: func(t *testing.T, m *OrgChartSectionResourceModel) {
				t.Helper()
				if m.CreatedAt.ValueString() != ca || m.UpdatedAt.ValueString() != ma {
					t.Fatalf("timestamps")
				}
				if m.Index.ValueInt64() != 2 {
					t.Fatalf("index")
				}
			},
		},
		{
			name: "nil_index",
			input: orgChartSectionAPIResponse{
				ID: "s2", Title: "S2",
			},
			assert: func(t *testing.T, m *OrgChartSectionResourceModel) {
				t.Helper()
				if !m.Index.IsNull() {
					t.Fatalf("index null")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m OrgChartSectionResourceModel
			mapOrgChartSectionResponseToState(&m, &tc.input)
			tc.assert(t, &m)
		})
	}
}

func TestOrgChartResource_Configure(t *testing.T) {
	t.Parallel()
	var r OrgChartResource
	var resp fwresource.ConfigureResponse
	r.Configure(context.Background(), fwresource.ConfigureRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no diagnostics for nil provider data")
	}

	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: "not-a-client"}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected error for wrong provider data type")
	}
}

func TestOrgChartSectionResource_Configure(t *testing.T) {
	t.Parallel()
	var r OrgChartSectionResource
	var resp fwresource.ConfigureResponse
	r.Configure(context.Background(), fwresource.ConfigureRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no diagnostics for nil provider data")
	}

	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: 42}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected error for wrong provider data type")
	}
}

func TestOrgChartResource_Metadata_Schema(t *testing.T) {
	t.Parallel()
	var r OrgChartResource
	var meta fwresource.MetadataResponse
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "langsmith"}, &meta)
	if want := "langsmith_org_chart"; meta.TypeName != want {
		t.Fatalf("TypeName = %q, want %q", meta.TypeName, want)
	}
	var schemaResp fwresource.SchemaResponse
	r.Schema(context.Background(), fwresource.SchemaRequest{}, &schemaResp)
	if schemaResp.Schema.Attributes == nil {
		t.Fatal("expected schema attributes")
	}
}

func TestOrgChartSectionResource_Metadata_Schema(t *testing.T) {
	t.Parallel()
	var r OrgChartSectionResource
	var meta fwresource.MetadataResponse
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "langsmith"}, &meta)
	if want := "langsmith_org_chart_section"; meta.TypeName != want {
		t.Fatalf("TypeName = %q, want %q", meta.TypeName, want)
	}
	var schemaResp fwresource.SchemaResponse
	r.Schema(context.Background(), fwresource.SchemaRequest{}, &schemaResp)
	if schemaResp.Schema.Attributes == nil {
		t.Fatal("expected schema attributes")
	}
}

// TestOrgChartClientCRUD exercises org chart HTTP flows with a mock transport.
func TestOrgChartClientCRUD(t *testing.T) {
	var chartID string
	rt := &mockRoundTripper{fn: func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/org-charts/create":
			body, _ := io.ReadAll(req.Body)
			var cr orgChartCreateRequest
			_ = json.Unmarshal(body, &cr)
			chartID = "chart-created"
			out := orgChartAPIResponse{
				ID: chartID, Title: cr.Title, ChartType: cr.ChartType,
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/org-charts/"+chartID:
			out := orgChartAPIResponse{
				ID: chartID, Title: "T", ChartType: "line",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodPatch && req.URL.Path == "/api/v1/org-charts/"+chartID:
			out := orgChartAPIResponse{
				ID: chartID, Title: "Updated", ChartType: "bar",
				Series: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`),
				CommonFilters: json.RawMessage(`{}`),
			}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/org-charts/"+chartID:
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}
		}
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("unexpected " + req.URL.Path)),
		}
	}}

	c := clientpkg.NewClient("http://example", "key", "", "org", "ua")
	c.HTTPClient.Transport = rt
	c.MaxRetries = 0
	ctx := context.Background()

	var created orgChartAPIResponse
	if err := c.Post(ctx, "/api/v1/org-charts/create", orgChartCreateRequest{
		Title: "N", ChartType: "line", Series: json.RawMessage(`[]`),
	}, &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != chartID {
		t.Fatalf("id = %q", created.ID)
	}

	var read orgChartAPIResponse
	readBody := struct {
		OmitData  bool   `json:"omit_data"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}{OmitData: true, StartTime: "2020-01-01T00:00:00Z", EndTime: "2020-01-01T00:01:00Z"}
	if err := c.Post(ctx, "/api/v1/org-charts/"+chartID, readBody, &read); err != nil {
		t.Fatal(err)
	}

	var updated orgChartAPIResponse
	if err := c.Patch(ctx, "/api/v1/org-charts/"+chartID, orgChartUpdateRequest{
		Title: strPtr("Updated"), ChartType: strPtr("bar"),
	}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated" {
		t.Fatalf("title %q", updated.Title)
	}

	if err := c.Delete(ctx, "/api/v1/org-charts/"+chartID); err != nil {
		t.Fatal(err)
	}
}

func strPtr(s string) *string { return &s }

func jsonResp(b []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(b)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// TestOrgChartSectionClientCRUD exercises org chart section HTTP flows with a mock transport.
func TestOrgChartSectionClientCRUD(t *testing.T) {
	secID := "sec-created"
	rt := &mockRoundTripper{fn: func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/org-charts/section":
			body, _ := io.ReadAll(req.Body)
			var cr orgChartSectionCreateRequest
			_ = json.Unmarshal(body, &cr)
			out := orgChartSectionAPIResponse{
				ID: secID, Title: cr.Title, Description: cr.Description, Index: cr.Index,
			}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/org-charts/section/"+secID:
			out := orgChartSectionAPIResponse{ID: secID, Title: "Sec"}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodPatch && req.URL.Path == "/api/v1/org-charts/section/"+secID:
			out := orgChartSectionAPIResponse{ID: secID, Title: "Patched"}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/org-charts/section/"+secID:
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}
		}
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("unexpected " + req.URL.Path)),
		}
	}}

	c := clientpkg.NewClient("http://example", "key", "", "org", "ua")
	c.HTTPClient.Transport = rt
	c.MaxRetries = 0
	ctx := context.Background()

	var created orgChartSectionAPIResponse
	if err := c.Post(ctx, "/api/v1/org-charts/section", orgChartSectionCreateRequest{Title: "S"}, &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != secID {
		t.Fatalf("id = %q", created.ID)
	}

	readBody := struct {
		OmitData  bool   `json:"omit_data"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}{OmitData: true, StartTime: "2020-01-01T00:00:00Z", EndTime: "2020-01-01T00:01:00Z"}
	var read orgChartSectionAPIResponse
	if err := c.Post(ctx, "/api/v1/org-charts/section/"+secID, readBody, &read); err != nil {
		t.Fatal(err)
	}

	var patched orgChartSectionAPIResponse
	if err := c.Patch(ctx, "/api/v1/org-charts/section/"+secID, orgChartSectionUpdateRequest{
		Title: strPtr("Patched"),
	}, &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Title != "Patched" {
		t.Fatalf("title %q", patched.Title)
	}

	if err := c.Delete(ctx, "/api/v1/org-charts/section/"+secID); err != nil {
		t.Fatal(err)
	}
}

// TestOrgChartSectionResource_framework exercises org chart section CRUD against a local HTTP server.
func TestOrgChartSectionResource_framework(t *testing.T) {
	var mu sync.Mutex
	sections := map[string]orgChartSectionAPIResponse{}
	next := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/org-charts/section":
			var body orgChartSectionCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := fmt.Sprintf("sec-%d", next)
			next++
			resp := orgChartSectionAPIResponse{
				ID: id, Title: body.Title, Description: body.Description, Index: body.Index,
			}
			sections[id] = resp
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/section/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/section/")
			sec, ok := sections[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sec)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/section/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/section/")
			var body orgChartSectionUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			sec := sections[id]
			if body.Title != nil {
				sec.Title = *body.Title
			}
			if body.Description != nil {
				sec.Description = body.Description
			}
			if body.Index != nil {
				sec.Index = body.Index
			}
			sections[id] = sec
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sec)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/section/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/section/")
			delete(sections, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)
	t.Setenv("LANGSMITH_ORGANIZATION_ID", "org-test")

	title := fmt.Sprintf("tf-os-%s", acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))
	titleUp := title + "-up"

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "langsmith" {
  organization_id = "  org-test  "
}
resource "langsmith_org_chart_section" "x" {
  title = %[1]q
}
`, title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_org_chart_section.x", "title", title),
					resource.TestCheckResourceAttrSet("langsmith_org_chart_section.x", "id"),
				),
			},
			{
				ResourceName:      "langsmith_org_chart_section.x",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: fmt.Sprintf(`
provider "langsmith" {
  organization_id = "  org-test  "
}
resource "langsmith_org_chart_section" "x" {
  title       = %[1]q
  description = "changed"
}
`, titleUp),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_org_chart_section.x", "title", titleUp),
					resource.TestCheckResourceAttr("langsmith_org_chart_section.x", "description", "changed"),
				),
			},
		},
	})
}

// TestOrgChartResource_framework exercises org chart CRUD with a section against a local HTTP server.
func TestOrgChartResource_framework(t *testing.T) {
	var mu sync.Mutex
	sections := map[string]orgChartSectionAPIResponse{}
	charts := map[string]orgChartAPIResponse{}
	nextSec, nextChart := 1, 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/org-charts/section":
			var body orgChartSectionCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := fmt.Sprintf("sec-%d", nextSec)
			nextSec++
			resp := orgChartSectionAPIResponse{ID: id, Title: body.Title, Description: body.Description, Index: body.Index}
			sections[id] = resp
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/section/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/section/")
			sec, ok := sections[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sec)

		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/section/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/section/")
			var body orgChartSectionUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			sec := sections[id]
			if body.Title != nil {
				sec.Title = *body.Title
			}
			sections[id] = sec
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sec)

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/section/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/section/")
			delete(sections, id)
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/org-charts/create":
			var body orgChartCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := fmt.Sprintf("chart-%d", nextChart)
			nextChart++
			meta := json.RawMessage(`null`)
			if body.Metadata != nil {
				meta = *body.Metadata
			}
			cf := json.RawMessage(`null`)
			if body.CommonFilters != nil {
				cf = *body.CommonFilters
			}
			series := body.Series
			if len(series) == 0 {
				series = json.RawMessage(`[]`)
			}
			resp := orgChartAPIResponse{
				ID: id, Title: body.Title, ChartType: body.ChartType,
				Series: series, Metadata: meta, CommonFilters: cf,
				Description: body.Description, Index: body.Index, SectionID: body.SectionID,
			}
			charts[id] = resp
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/")
			if id == "create" || strings.HasPrefix(id, "section") {
				http.Error(w, "bad path", http.StatusBadRequest)
				return
			}
			ch, ok := charts[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ch)

		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/")
			var body orgChartUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			ch := charts[id]
			if body.Title != nil {
				ch.Title = *body.Title
			}
			if body.ChartType != nil {
				ch.ChartType = *body.ChartType
			}
			if body.Metadata != nil {
				ch.Metadata = *body.Metadata
			}
			if body.CommonFilters != nil {
				ch.CommonFilters = *body.CommonFilters
			}
			if len(body.Series) > 0 {
				ch.Series = body.Series
			}
			charts[id] = ch
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ch)

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/org-charts/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/org-charts/")
			delete(charts, id)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)
	t.Setenv("LANGSMITH_ORGANIZATION_ID", "org-test")

	secTitle := fmt.Sprintf("tf-sec-%s", acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))
	chartTitle := fmt.Sprintf("tf-ch-%s", acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "langsmith" {
  organization_id = "  org-test  "
}
resource "langsmith_org_chart_section" "s" {
  title = %[1]q
}
resource "langsmith_org_chart" "c" {
  title       = %[2]q
  chart_type  = "line"
  section_id  = langsmith_org_chart_section.s.id
  series = jsonencode([
    {
      name   = "Run Count"
      metric = "run_count"
    }
  ])
  description = "d0"
  metadata    = jsonencode({ "k" = "v" })
}
`, secTitle, chartTitle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "title", chartTitle),
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "chart_type", "line"),
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "description", "d0"),
				),
			},
			{
				ResourceName:            "langsmith_org_chart.c",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"series", "section_id", "metadata"},
			},
			{
				Config: fmt.Sprintf(`
provider "langsmith" {
  organization_id = "  org-test  "
}
resource "langsmith_org_chart_section" "s" {
  title = %[1]q
}
resource "langsmith_org_chart" "c" {
  title      = %[2]q
  chart_type = "bar"
  section_id = langsmith_org_chart_section.s.id
  series = jsonencode([
    {
      name   = "Run Count"
      metric = "run_count"
    }
  ])
  metadata    = jsonencode({ "k" = "v" })
  description = "d0"
}
`, secTitle, chartTitle+"-2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "title", chartTitle+"-2"),
					resource.TestCheckResourceAttr("langsmith_org_chart.c", "chart_type", "bar"),
				),
			},
		},
	})
}
