// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPlatformFeatureResource_contract(t *testing.T) {
	var mu sync.Mutex
	defaultModel := ""
	disabled := map[string]struct{}{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/features":
			dm := defaultModel
			var dmPtr *string
			if dm != "" {
				dmPtr = &dm
			}
			dis := make([]string, 0, len(disabled))
			for m := range disabled {
				dis = append(dis, m)
			}
			out := []map[string]any{
				{
					"feature":         "playground",
					"default_model":   dmPtr,
					"disabled_models": dis,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		case r.Method == http.MethodPut && r.URL.Path == "/v1/platform/features/playground/default-model":
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defaultModel = payload.Model
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/platform/features/playground/default-model":
			defaultModel = ""
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodPut && r.URL.Path == "/v1/platform/features/playground/disabled-models":
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			disabled[payload.Model] = struct{}{}
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/platform/features/playground/disabled-models/"):
			suffix := strings.TrimPrefix(r.URL.Path, "/v1/platform/features/playground/disabled-models/")
			delete(disabled, suffix)
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
resource "langsmith_platform_feature" "pf" {
  feature         = "playground"
  default_model     = "claude-3-sonnet"
  disabled_models   = ["gpt-4", "gpt-4o"]
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "id", "playground"),
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "feature", "playground"),
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "default_model", "claude-3-sonnet"),
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "disabled_models.#", "2"),
				),
			},
			{
				ResourceName:      "langsmith_platform_feature.pf",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccPlatformFeatureResource_contract_preserveUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/features":
			dm := "keep-me"
			out := []map[string]any{
				{
					"feature":         "playground",
					"default_model":   dm,
					"disabled_models": []string{"leave-alone"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/platform/features/playground/default-model":
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/platform/features/playground/disabled-models/"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
resource "langsmith_platform_feature" "pf" {
  feature = "playground"
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "default_model", "keep-me"),
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "disabled_models.#", "1"),
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "disabled_models.0", "leave-alone"),
				),
			},
		},
	})
}

func TestAccPlatformFeatureResource_contract_preserveNullDisabledModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/features":
			dm := "keep-me"
			out := []map[string]any{
				{
					"feature":         "playground",
					"default_model":   dm,
					"disabled_models": []string{"leave-alone"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/platform/features/playground/default-model":
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/platform/features/playground/disabled-models/"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
resource "langsmith_platform_feature" "pf" {
  feature = "playground"
  disabled_models = null
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "default_model", "keep-me"),
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "disabled_models.#", "1"),
					resource.TestCheckResourceAttr("langsmith_platform_feature.pf", "disabled_models.0", "leave-alone"),
				),
			},
		},
	})
}

func TestAccPlatformFeaturesDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/features":
			a := "m-a"
			out := []map[string]any{
				{"feature": "zebra", "default_model": nil, "disabled_models": []string{}},
				{"feature": "alpha", "default_model": &a, "disabled_models": []string{"x", "y"}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_platform_features" "x" {}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_platform_features.x", "features.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_platform_features.x", "features.0.feature", "alpha"),
					resource.TestCheckResourceAttr("data.langsmith_platform_features.x", "features.0.default_model", "m-a"),
					resource.TestCheckResourceAttr("data.langsmith_platform_features.x", "features.0.disabled_models.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_platform_features.x", "features.1.feature", "zebra"),
				),
			},
		},
	})
}
