// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccInsightsConfigResource_contract(t *testing.T) {
	sess := "sess-eeeeeeee-eeee-eeee-eeeeeeeeeeee"
	cfgID := "insig-ffffffff-ffff-ffff-ffffffffffff"

	var mu sync.Mutex
	configs := []insightsConfigAPI{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/sessions/" + sess + "/insights/configs"
		pathOne := base + "/" + cfgID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodPost && r.URL.Path == base:
			var body insightsConfigCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			item := insightsConfigAPI{
				ID: cfgID, Name: body.Name, Description: body.Description,
				Config: body.Config, ScheduleCron: body.ScheduleCron,
			}
			configs = append(configs, item)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(item)

		case r.Method == http.MethodGet && r.URL.Path == base:
			mu.Lock()
			list := make([]insightsConfigAPI, len(configs))
			copy(list, configs)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(getInsightsConfigsResponse{Configs: list})

		case r.Method == http.MethodPatch && r.URL.Path == pathOne:
			body, _ := io.ReadAll(r.Body)
			var u insightsConfigUpdateRequest
			_ = json.Unmarshal(body, &u)
			mu.Lock()
			var out insightsConfigAPI
			for i := range configs {
				if configs[i].ID == cfgID {
					if u.Name != nil {
						configs[i].Name = *u.Name
					}
					out = configs[i]
					break
				}
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)

		case r.Method == http.MethodDelete && r.URL.Path == pathOne:
			mu.Lock()
			filtered := configs[:0]
			for _, c := range configs {
				if c.ID != cfgID {
					filtered = append(filtered, c)
				}
			}
			configs = filtered
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
resource "langsmith_insights_config" "i" {
  session_id = "` + sess + `"
  name       = "job-a"
  config     = jsonencode({ model = { name = "m" } })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_insights_config.i", "id", cfgID),
				),
			},
			{
				Config: `
resource "langsmith_insights_config" "i" {
  session_id = "` + sess + `"
  name       = "job-b"
  config     = jsonencode({ model = { name = "m" } })
}
`,
			},
		},
	})
}

func TestAccDatasetSplitResource_contract(t *testing.T) {
	ds := "ds-12121212-1212-1212-121212121212"
	var mu sync.Mutex
	examples := map[string]struct{}{} // example IDs in split "train"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/api/v1/datasets/" + ds + "/splits"
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodPut && r.URL.Path == path:
			var sm splitMutation
			if err := json.NewDecoder(r.Body).Decode(&sm); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if sm.SplitName != "train" {
				http.Error(w, "only train", http.StatusBadRequest)
				return
			}
			mu.Lock()
			if sm.Remove {
				for _, e := range sm.Examples {
					delete(examples, e)
				}
			} else {
				for _, e := range sm.Examples {
					examples[e] = struct{}{}
				}
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]string{})

		case r.Method == http.MethodGet && r.URL.Path == path:
			mu.Lock()
			has := len(examples) > 0
			mu.Unlock()
			var names []string
			if has {
				names = []string{"train"}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(names)

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
resource "langsmith_dataset_split" "s" {
  dataset_id   = "` + ds + `"
  name         = "train"
  example_ids  = ["ex-1", "ex-2"]
}
`,
			},
			{
				Config: `
resource "langsmith_dataset_split" "s" {
  dataset_id   = "` + ds + `"
  name         = "train"
  example_ids  = ["ex-1", "ex-2", "ex-3"]
}
`,
			},
		},
	})
}
