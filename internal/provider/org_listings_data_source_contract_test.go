// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationsDataSource_contract(t *testing.T) {
	orgPayload := `[{"id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","display_name":"Beta Org","tier":"enterprise","is_personal":false,"disabled":false,"ip_allowlist":["10.0.0.0/8","192.168.0.0/16","10.0.0.0/8"]},{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","display_name":"Alpha Org","tier":"plus","is_personal":true,"disabled":false,"sso_only":true,"invites_enabled":false}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(orgPayload))
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_organizations" "o" {}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "id", "organizations"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.0.id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.0.display_name", "Alpha Org"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.0.tier", "plus"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.0.is_personal", "true"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.0.sso_only", "true"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.0.invites_enabled", "false"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.1.id", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.1.ip_allowlist.#", "3"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.1.ip_allowlist.0", "10.0.0.0/8"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.1.ip_allowlist.1", "10.0.0.0/8"),
					resource.TestCheckResourceAttr("data.langsmith_organizations.o", "organizations.1.ip_allowlist.2", "192.168.0.0/16"),
				),
			},
		},
	})
}

func TestAccOrganizationPendingInvitesDataSource_contract(t *testing.T) {
	pendingPayload := `[{"id":"cccccccc-cccc-cccc-cccc-cccccccccccc","display_name":"Invited Org","is_personal":false,"disabled":false}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/pending":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(pendingPayload))
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_organization_pending_invites" "p" {}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_organization_pending_invites.p", "id", "organization_pending_invites"),
					resource.TestCheckResourceAttr("data.langsmith_organization_pending_invites.p", "pending_invites.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_organization_pending_invites.p", "pending_invites.0.display_name", "Invited Org"),
				),
			},
		},
	})
}

func TestAccOrganizationPermissionsDataSource_contract(t *testing.T) {
	permPayload := `[{"name":"workspace:read","description":"Read workspace","access_scope":"workspace"},{"name":"organization:admin","description":"Org admin","access_scope":"organization"}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/permissions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(permPayload))
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_organization_permissions" "perm" {}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_organization_permissions.perm", "id", "organization_permissions"),
					resource.TestCheckResourceAttr("data.langsmith_organization_permissions.perm", "permissions.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_organization_permissions.perm", "permissions.0.name", "organization:admin"),
					resource.TestCheckResourceAttr("data.langsmith_organization_permissions.perm", "permissions.0.access_scope", "organization"),
					resource.TestCheckResourceAttr("data.langsmith_organization_permissions.perm", "permissions.1.name", "workspace:read"),
					resource.TestCheckResourceAttr("data.langsmith_organization_permissions.perm", "permissions.1.access_scope", "workspace"),
				),
			},
		},
	})
}
