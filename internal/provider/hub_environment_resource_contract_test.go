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
)

func TestAccHubEnvironmentResource_contract(t *testing.T) {
	const hubID = "hub-rec-11111111-1111-4111-8111-111111111111"

	var mu sync.Mutex
	stored := hubEnvAPI{ID: hubID, Environments: []hubEnvEntry{{Name: "alpha"}, {Name: "beta"}}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/hub/environments":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var req hubEnvRequest
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			stored = hubEnvAPI{ID: hubID, Environments: req.Environments}
			out := stored
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/hub/environments":
			mu.Lock()
			out := stored
			mu.Unlock()
			if out.ID == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return

		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/hub/environments/"+hubID:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var req hubEnvRequest
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			stored.Environments = req.Environments
			out := stored
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/hub/environments/"+hubID:
			mu.Lock()
			stored = hubEnvAPI{}
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfgTwo := `
resource "langsmith_hub_environment" "h" {
  environments = [
    { name = "alpha" },
    { name = "beta" },
  ]
}
`
	cfgThree := `
resource "langsmith_hub_environment" "h" {
  environments = [
    { name = "alpha" },
    { name = "beta" },
    { name = "gamma" },
  ]
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfgTwo,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_hub_environment.h", "id", hubID),
					resource.TestCheckResourceAttr("langsmith_hub_environment.h", "environments.#", "2"),
				),
			},
			{
				Config: cfgThree,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_hub_environment.h", "environments.#", "3"),
					resource.TestCheckResourceAttr("langsmith_hub_environment.h", "environments.2.name", "gamma"),
				),
			},
			{
				ResourceName:      "langsmith_hub_environment.h",
				ImportState:       true,
				ImportStateId:     hubID,
				ImportStateVerify: true,
			},
		},
	})
}
