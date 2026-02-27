// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccUserDataSource_byEmail verifies the user data source can look up a
// user by email and return the canonical user ID.
func TestAccUserDataSource_byEmail(t *testing.T) {
	email := os.Getenv("LANGSMITH_TEST_USER_EMAIL")
	if email == "" {
		t.Skip("LANGSMITH_TEST_USER_EMAIL not set; skipping acceptance test")
	}

	cfg := "data \"langsmith_user\" \"test\" {\n  email = \"" + email + "\"\n}"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_user.test", "id"),
					resource.TestCheckResourceAttrSet("data.langsmith_user.test", "display_name"),
				),
			},
		},
	})
}

// TestAccUserDataSource_framework runs the user data source against a local
// HTTP test server that simulates the LangSmith API so we can unit-test the
// lookup behavior without external credentials.
func TestAccUserDataSource_framework(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/current/members":
			w.Header().Set("Content-Type", "application/json")
			// return a members list containing our test user
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"members": []map[string]interface{}{{"id": "m-1", "user_id": "u-1", "email": "user@example.com", "full_name": "Test User"}}})
			return
		default:
			http.Error(w, "not found", 404)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_user" "test" { email = "user@example.com" }`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_user.test", "id", "u-1"),
					resource.TestCheckResourceAttr("data.langsmith_user.test", "display_name", "Test User"),
				),
			},
		},
	})
}
