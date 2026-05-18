// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPersonalAccessTokenResource_contract(t *testing.T) {
	const patID = "pat-4444-4444-8444-444444444444"
	var mu sync.Mutex
	tokens := []patListItem{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs/current/personal-access-tokens":
			var body patCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			tokens = append(tokens, patListItem{
				ID: patID, Description: body.Description, ShortKey: "sk_test",
				CreatedAt: "2026-03-01T00:00:00Z",
			})
			mu.Unlock()
			out := patCreateResponse{
				ID: patID, Description: body.Description, ShortKey: "sk_test",
				Key: "full-secret-key", CreatedAt: "2026-03-01T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/current/personal-access-tokens":
			mu.Lock()
			list := make([]patListItem, len(tokens))
			copy(list, tokens)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/orgs/current/personal-access-tokens/"+patID:
			mu.Lock()
			filtered := tokens[:0]
			for _, t := range tokens {
				if t.ID != patID {
					filtered = append(filtered, t)
				}
			}
			tokens = filtered
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

	cfg := `
resource "langsmith_personal_access_token" "p" {
  description = "contract PAT"
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_personal_access_token.p", "id", patID),
					resource.TestCheckResourceAttr("langsmith_personal_access_token.p", "description", "contract PAT"),
					resource.TestCheckResourceAttr("langsmith_personal_access_token.p", "short_key", "sk_test"),
					resource.TestCheckResourceAttr("langsmith_personal_access_token.p", "key", "full-secret-key"),
					resource.TestCheckResourceAttr("langsmith_personal_access_token.p", "created_at", "2026-03-01T00:00:00Z"),
				),
			},
		},
	})
}
