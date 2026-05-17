// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAPIKeyResource_framework runs langsmith_api_key against a local HTTP
// server that models the OpenAPI contract for /api/v1/api-key.
func TestAPIKeyResource_framework(t *testing.T) {
	const testKeyID = "11111111-1111-1111-1111-111111111111"

	var mu sync.Mutex
	var stored *apiKeyCreateResponse

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/api-key":
			mu.Lock()
			defer mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if stored == nil {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_ = json.NewEncoder(w).Encode([]apiKeyGetResponse{{
				ID:          stored.ID,
				ShortKey:    stored.ShortKey,
				Description: stored.Description,
				ReadOnly:    stored.ReadOnly,
				CreatedAt:   stored.CreatedAt,
				LastUsedAt:  stored.LastUsedAt,
				ExpiresAt:   stored.ExpiresAt,
			}})

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/api-key":
			var body apiKeyCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			created := &apiKeyCreateResponse{
				ID:          testKeyID,
				ShortKey:    "lsv2_****abcd",
				Description: body.Description,
				ReadOnly:    body.ReadOnly,
				Key:         "lsv2_test_secret_value",
				CreatedAt:   ptr("2026-01-02T15:04:05Z"),
			}
			mu.Lock()
			stored = created
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(created)
			return

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/api-key/"+testKeyID:
			mu.Lock()
			stored = nil
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"id": testKeyID})
			return

		default:
			http.Error(w, fmt.Sprintf("not found: %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
resource "langsmith_api_key" "test" {
  description = "contract mock"
  read_only     = false
}
`

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_api_key.test", "id", testKeyID),
					resource.TestCheckResourceAttr("langsmith_api_key.test", "short_key", "lsv2_****abcd"),
					resource.TestCheckResourceAttr("langsmith_api_key.test", "key", "lsv2_test_secret_value"),
					resource.TestCheckResourceAttr("langsmith_api_key.test", "description", "contract mock"),
				),
			},
			{
				ResourceName:            "langsmith_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
		},
	})
}

func ptr(s string) *string {
	return &s
}
