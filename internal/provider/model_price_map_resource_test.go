// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestModelPriceMapResource_matchPathComputed guards #89. LangSmith fills in a
// default match_path when it is omitted, so the attribute must be Computed:
// as Optional alone, the first apply failed with "Provider produced
// inconsistent result after apply: .match_path: was null, but now ...".
func TestModelPriceMapResource_matchPathComputed(t *testing.T) {
	var resp resource.SchemaResponse
	NewModelPriceMapResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["match_path"].(schema.ListAttribute)
	if !ok {
		t.Fatalf("match_path is %T, want schema.ListAttribute", resp.Schema.Attributes["match_path"])
	}
	if !attr.Optional || !attr.Computed {
		t.Fatalf("match_path Optional=%v Computed=%v, want both true", attr.Optional, attr.Computed)
	}
	if len(attr.PlanModifiers) == 0 {
		t.Fatal("match_path has no plan modifiers, want UseStateForUnknown so an unchanged value does not plan as unknown")
	}
}

// modelPriceMapDefaultMatchPath is what LangSmith fills in when match_path is
// omitted.
var modelPriceMapDefaultMatchPath = []string{"model", "model_name", "model_id", "model_path", "endpoint_name"}

// modelPriceMapStub emulates the model-price-map endpoints with the two server
// behaviors behind #89 and #90: an omitted match_path comes back as the
// server default, and cost-detail objects come back re-serialized, with
// numbers in scientific notation and keys in reverse order. It needs no
// credentials; the provider is pointed at it via LANGSMITH_API_URL.
type modelPriceMapStub struct {
	mu      sync.Mutex
	entries map[string]map[string]any
	nextID  int
}

// reserialize re-encodes a cost-details object the way the API spells it.
// jsonencode() writes 0.0000001; the API returns 1e-07.
func (s *modelPriceMapStub) reserialize(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var m map[string]float64
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q: %s", k, strconv.FormatFloat(m[k], 'e', -1, 64)))
	}
	return json.RawMessage("{" + strings.Join(parts, ", ") + "}")
}

func (s *modelPriceMapStub) store(id string, req modelPriceMapAPIRequest) map[string]any {
	matchPath := req.MatchPath
	if len(matchPath) == 0 {
		matchPath = modelPriceMapDefaultMatchPath
	}
	entry := map[string]any{
		"id":                      id,
		"name":                    req.Name,
		"match_pattern":           req.MatchPattern,
		"prompt_cost":             req.PromptCost,
		"completion_cost":         req.CompletionCost,
		"provider":                req.Provider,
		"start_time":              req.StartTime,
		"match_path":              matchPath,
		"prompt_cost_details":     s.reserialize(req.PromptCostDetails),
		"completion_cost_details": s.reserialize(req.CompletionCostDetails),
	}
	s.entries[id] = entry
	return entry
}

func (s *modelPriceMapStub) start(t *testing.T) {
	t.Helper()
	s.entries = map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		const base = "/api/v1/model-price-map"

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodPost && r.URL.Path == base:
			var req modelPriceMapAPIRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			s.nextID++
			_ = json.NewEncoder(w).Encode(s.store(fmt.Sprintf("price-%d", s.nextID), req))

		case r.Method == http.MethodGet && r.URL.Path == base:
			list := make([]map[string]any, 0, len(s.entries))
			for _, e := range s.entries {
				list = append(list, e)
			}
			_ = json.NewEncoder(w).Encode(list)

		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, base+"/"):
			id := strings.TrimPrefix(r.URL.Path, base+"/")
			var req modelPriceMapAPIRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(s.store(id, req))

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, base+"/"):
			delete(s.entries, strings.TrimPrefix(r.URL.Path, base+"/"))
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, fmt.Sprintf("unexpected %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-test")
	t.Setenv("LANGSMITH_API_URL", srv.URL)
}

// TestAccModelPriceMapResource_serverDefaultsConverge is the end-to-end
// regression test for #89 and #90. It runs against a stub of the API, so it
// needs TF_ACC but no credentials.
//
// With match_path omitted and cost details written in decimal by
// jsonencode(), the first apply used to fail on match_path ("was null, but
// now ..."), and once that was worked around, every later plan showed a
// cost-details diff (1e-07 -> 0.0000001) that never converged. Here the apply
// must succeed and the plans that follow it must be empty. The framework
// already checks for an empty plan after each apply, with and without a
// refresh; the PlanOnly step repeats that after a fresh Read, which is where
// #90 bit.
func TestAccModelPriceMapResource_serverDefaultsConverge(t *testing.T) {
	stub := &modelPriceMapStub{}
	stub.start(t)

	const config = `
resource "langsmith_model_price_map" "test" {
  name            = "example/model"
  match_pattern   = "^example-model$"
  prompt_cost     = 0.000001
  completion_cost = 0.000005

  # match_path intentionally omitted; the API fills in its default.

  prompt_cost_details = jsonencode({
    cache_read     = 0.0000001
    cache_creation = 0.00000125
  })
  completion_cost_details = jsonencode({
    reasoning = 0.000005
  })
}
`
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: config,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("langsmith_model_price_map.test", "match_path.#", "5"),
					tfresource.TestCheckResourceAttr("langsmith_model_price_map.test", "match_path.0", "model"),
					tfresource.TestCheckResourceAttr("langsmith_model_price_map.test", "match_path.4", "endpoint_name"),
					// The configured spelling is kept, not the API's.
					tfresource.TestCheckResourceAttr("langsmith_model_price_map.test", "prompt_cost_details",
						`{"cache_creation":0.00000125,"cache_read":0.0000001}`),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}
