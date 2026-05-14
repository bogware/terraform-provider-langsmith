// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"fmt"
	"net/http/httptest"
	"strings"
	"sync"

	clientpkg "github.com/bogware/terraform-provider-langsmith/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccOrgRoleResource_basic is intentionally left as an acceptance test
// placeholder — acceptance tests require real credentials and are skipped
// in unit test runs. The unit tests below cover the resource mapping and
// basic CRUD HTTP flows using a RoundTripper mock.
func TestAccOrgRoleResource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
}

func TestMapOrgRoleResponseToState_FullAndEmptyDescription(t *testing.T) {
	perms := json.RawMessage(`[{"action":"read"}]`)

	cases := []struct {
		name     string
		input    orgRoleAPIResponse
		wantDesc types.String
	}{
		{
			name: "full",
			input: orgRoleAPIResponse{
				ID:             "r-123",
				Name:           "role_internal",
				DisplayName:    "Deputy",
				Description:    "Holds the fort",
				OrganizationID: "org-1",
				Permissions:    perms,
				AccessScope:    "organization",
			},
			wantDesc: types.StringValue("Holds the fort"),
		},
		{
			name: "empty description",
			input: orgRoleAPIResponse{
				ID:             "r-456",
				Name:           "role_internal_2",
				DisplayName:    "Marshal",
				Description:    "",
				OrganizationID: "org-2",
				Permissions:    perms,
				AccessScope:    "organization",
			},
			wantDesc: types.StringNull(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var m OrgRoleResourceModel
			mapOrgRoleResponseToState(&m, &tc.input)

			if got := m.ID.ValueString(); got != tc.input.ID {
				t.Fatalf("ID = %q, want %q", got, tc.input.ID)
			}
			if got := m.DisplayName.ValueString(); got != tc.input.DisplayName {
				t.Fatalf("DisplayName = %q, want %q", got, tc.input.DisplayName)
			}
			if got := m.Name.ValueString(); got != tc.input.Name {
				t.Fatalf("Name = %q, want %q", got, tc.input.Name)
			}
			if got := m.OrganizationID.ValueString(); got != tc.input.OrganizationID {
				t.Fatalf("OrganizationID = %q, want %q", got, tc.input.OrganizationID)
			}
			if got := m.AccessScope.ValueString(); got != tc.input.AccessScope {
				t.Fatalf("AccessScope = %q, want %q", got, tc.input.AccessScope)
			}

			// Description: either null or equals expected
			if tc.wantDesc.IsNull() {
				if !m.Description.IsNull() {
					t.Fatalf("Description should be null")
				}
			} else {
				if m.Description.ValueString() != tc.wantDesc.ValueString() {
					t.Fatalf("Description = %q, want %q", m.Description.ValueString(), tc.wantDesc.ValueString())
				}
			}

			// Permissions should be normalized JSON string
			if m.Permissions.IsNull() {
				t.Fatalf("Permissions should not be null")
			}
			// ensure it's valid JSON
			if !json.Valid([]byte(m.Permissions.ValueString())) {
				t.Fatalf("Permissions is not valid JSON: %s", m.Permissions.ValueString())
			}
		})
	}
}

func TestMapOrgRoleResponseToState_PreserveEmptyAccessScope(t *testing.T) {
	// Start with an existing state that has a non-empty access scope.
	var m OrgRoleResourceModel
	m.AccessScope = types.StringValue("workspace")

	// API returns a response with an empty access_scope (common on update).
	res := orgRoleAPIResponse{
		ID:             "r-1",
		Name:           "role1",
		DisplayName:    "Role One",
		Description:    "",
		OrganizationID: "org-1",
		Permissions:    json.RawMessage(`[{"action":"read"}]`),
		AccessScope:    "",
	}

	mapOrgRoleResponseToState(&m, &res)

	if got := m.AccessScope.ValueString(); got != "workspace" {
		t.Fatalf("AccessScope was overwritten; got %q want %q", got, "workspace")
	}
}

// TestOrgRoleClientCRUD exercises the HTTP interactions used by the
// org role resource using a mock RoundTripper. It verifies the client
// correctly marshals requests and unmarshals responses and that the
// mapping helper produces the expected state values.
func TestOrgRoleClientCRUD(t *testing.T) {
	// stateful created role stored in the handler closure
	var created orgRoleAPIResponse

	rt := &mockRoundTripper{fn: func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/orgs/current/roles":
			// create
			body, _ := io.ReadAll(req.Body)
			var cr orgRoleCreateRequest
			_ = json.Unmarshal(body, &cr)
			created = orgRoleAPIResponse{
				ID:             "r-created",
				Name:           "role_created",
				DisplayName:    cr.DisplayName,
				Description:    "",
				OrganizationID: "org-42",
				Permissions:    cr.Permissions,
				AccessScope:    "organization",
			}
			b, _ := json.Marshal(created)
			return &http.Response{StatusCode: 201, Body: io.NopCloser(bytes.NewReader(b)), Header: map[string][]string{"Content-Type": {"application/json"}}}

		case req.Method == http.MethodGet && req.URL.Path == "/api/v1/orgs/current/roles":
			// list
			arr := []orgRoleAPIResponse{}
			if created.ID != "" {
				arr = append(arr, created)
			}
			b, _ := json.Marshal(arr)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b)), Header: map[string][]string{"Content-Type": {"application/json"}}}

		case req.Method == http.MethodPatch:
			// update
			// path ends with id
			body, _ := io.ReadAll(req.Body)
			var ur orgRoleUpdateRequest
			_ = json.Unmarshal(body, &ur)
			created.DisplayName = ur.DisplayName
			if ur.Description != nil {
				created.Description = *ur.Description
			}
			created.Permissions = ur.Permissions
			b, _ := json.Marshal(created)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b)), Header: map[string][]string{"Content-Type": {"application/json"}}}

		case req.Method == http.MethodDelete:
			// delete
			created = orgRoleAPIResponse{}
			return &http.Response{StatusCode: 204, Body: io.NopCloser(bytes.NewReader([]byte{}))}
		}
		return &http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewReader([]byte("unexpected request")))}
	}}

	c := clientpkg.NewClient("http://example", "key", "tenant", "", "ua")
	c.HTTPClient.Transport = rt
	c.MaxRetries = 0

	ctx := context.Background()

	// Create
	var createResp orgRoleAPIResponse
	createBody := orgRoleCreateRequest{
		DisplayName: "New Role",
		Permissions: json.RawMessage(`[{"action":"read"}]`),
	}
	if err := c.Post(ctx, "/api/v1/orgs/current/roles", createBody, &createResp); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if createResp.ID == "" {
		t.Fatalf("expected created ID")
	}

	// Read (list) and map
	var listResp orgRoleListAPIResponse
	if err := c.Get(ctx, "/api/v1/orgs/current/roles", nil, &listResp); err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if len(listResp) != 1 {
		t.Fatalf("expected 1 role in list, got %d", len(listResp))
	}
	var state OrgRoleResourceModel
	mapOrgRoleResponseToState(&state, &listResp[0])
	if state.ID.ValueString() != createResp.ID {
		// these IDs come from the mock; ensure mapping worked
		t.Fatalf("mapped ID = %q, want %q", state.ID.ValueString(), createResp.ID)
	}

	// Update
	var updateResp orgRoleAPIResponse
	upBody := orgRoleUpdateRequest{
		DisplayName: "Updated Role",
		Permissions: json.RawMessage(`[{"action":"read"}]`),
	}
	if err := c.Patch(ctx, "/api/v1/orgs/current/roles/"+createResp.ID, upBody, &updateResp); err != nil {
		t.Fatalf("Patch error: %v", err)
	}
	if updateResp.DisplayName != "Updated Role" {
		t.Fatalf("update display name = %q, want %q", updateResp.DisplayName, "Updated Role")
	}

	// Delete
	if err := c.Delete(ctx, "/api/v1/orgs/current/roles/"+createResp.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	// verify list is empty
	if err := c.Get(ctx, "/api/v1/orgs/current/roles", nil, &listResp); err != nil {
		t.Fatalf("Get after delete error: %v", err)
	}
	if len(listResp) != 0 {
		t.Fatalf("expected 0 roles after delete, got %d", len(listResp))
	}
}

// mockRoundTripper is a tiny helper to stub HTTP requests in tests.
type mockRoundTripper struct {
	fn func(*http.Request) *http.Response
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req), nil
}

// TestAccOrgRoleResource_framework runs the provider against a local HTTP test
// server that simulates the LangSmith API. This uses the Terraform provider
// testing framework but does not require external credentials.
func TestAccOrgRoleResource_framework(t *testing.T) {
	var mu sync.Mutex
	roles := map[string]orgRoleAPIResponse{}
	nextID := 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs/current/roles":
			var cr orgRoleCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&cr)
			id := fmt.Sprintf("r-%d", nextID)
			nextID++
			resp := orgRoleAPIResponse{
				ID:             id,
				Name:           "role_" + id,
				DisplayName:    cr.DisplayName,
				Description:    "",
				OrganizationID: "org-test",
				Permissions:    cr.Permissions,
				AccessScope:    "organization",
			}
			roles[id] = resp
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(resp)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/current/roles":
			list := []orgRoleAPIResponse{}
			for _, v := range roles {
				list = append(list, v)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/orgs/current/roles/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/orgs/current/roles/")
			var ur orgRoleUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&ur)
			rp := roles[id]
			rp.DisplayName = ur.DisplayName
			if ur.Description != nil {
				rp.Description = *ur.Description
			}
			rp.Permissions = ur.Permissions
			roles[id] = rp
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rp)
			return
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/orgs/current/roles/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/orgs/current/roles/")
			delete(roles, id)
			w.WriteHeader(204)
			return
		default:
			http.Error(w, "not found", 404)
			return
		}
	}))
	defer srv.Close()

	// Set env vars expected by the provider factory.
	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(s *terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: testAccOrgRoleConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_org_role.test", "id"),
					resource.TestCheckResourceAttr("langsmith_org_role.test", "display_name", rName),
				),
			},
			{
				Config: testAccOrgRoleConfig(rName + "-updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_org_role.test", "display_name", rName+"-updated"),
				),
			},
		},
	})
}

func testAccOrgRoleConfig(name string) string {
	return fmt.Sprintf(`resource "langsmith_org_role" "test" {
  display_name = %q
  permissions  = jsonencode([{"action":"read"}])
}
`, name)
}
