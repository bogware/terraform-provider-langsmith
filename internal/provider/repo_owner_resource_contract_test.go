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

func TestAccRepoOwnerResource_contract(t *testing.T) {
	const (
		owner      = "acme"
		repo       = "widget"
		email      = "owner@example.com"
		identityID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	)
	apiPath := "/api/v1/repos/" + owner + "/" + repo + "/owners"

	var mu sync.Mutex
	owners := []repoOwnerAPI{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == apiPath:
			var body addRepoOwnerRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			em := body.Email
			fn := "Pat Owner"
			mu.Lock()
			owners = append(owners, repoOwnerAPI{
				IdentityID: strPtr(identityID),
				LSUserID:   "ls-user-1",
				Email:      &em,
				FullName:   &fn,
				CreatedAt:  "2026-01-15T10:00:00Z",
			})
			mu.Unlock()
			last := owners[len(owners)-1]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(last)
			return

		case r.Method == http.MethodGet && r.URL.Path == apiPath:
			mu.Lock()
			list := make([]repoOwnerAPI, len(owners))
			copy(list, owners)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(listRepoOwnersResponse{Owners: list})
			return

		case r.Method == http.MethodDelete && r.URL.Path == apiPath:
			b, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var del removeRepoOwnerRequest
			if err := json.Unmarshal(b, &del); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			filtered := owners[:0]
			for _, o := range owners {
				if o.IdentityID != nil && *o.IdentityID == del.IdentityID {
					continue
				}
				filtered = append(filtered, o)
			}
			owners = filtered
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
resource "langsmith_repo_owner" "o" {
  owner = "` + owner + `"
  repo  = "` + repo + `"
  email = "` + email + `"
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_repo_owner.o", "id", identityID),
					resource.TestCheckResourceAttr("langsmith_repo_owner.o", "identity_id", identityID),
					resource.TestCheckResourceAttr("langsmith_repo_owner.o", "ls_user_id", "ls-user-1"),
					resource.TestCheckResourceAttr("langsmith_repo_owner.o", "email", email),
					resource.TestCheckResourceAttr("langsmith_repo_owner.o", "full_name", "Pat Owner"),
					resource.TestCheckResourceAttr("langsmith_repo_owner.o", "created_at", "2026-01-15T10:00:00Z"),
				),
			},
		},
	})
}

func strPtr(s string) *string { return &s }
