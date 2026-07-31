// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	clientpkg "github.com/bogware/terraform-provider-langsmith/internal/client"
)

// TestDeleteDataPlane covers the destroy path against a stub server. A
// deployment that does not expose the delete endpoint must not be able to wedge
// a destroy, so 404 and 405 have to degrade to a warning while a genuine
// failure still surfaces as an error.
func TestDeleteDataPlane(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantErr     bool
		wantWarning bool
	}{
		{name: "deleted", status: 200, body: `{}`},
		{name: "route or object absent", status: 404, body: `{"detail":"not found"}`, wantWarning: true},
		{name: "endpoint not supported", status: 405, body: `{"detail":"method not allowed"}`, wantWarning: true},
		{name: "forbidden surfaces", status: 403, body: `{"detail":"nope"}`, wantErr: true},
		{name: "server error surfaces", status: 500, body: `boom`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				gotPath, gotMethod = req.URL.Path, req.Method
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := clientpkg.NewClient(srv.URL, "key", "workspace", "ua", false, nil)
			c.MaxRetries = 0
			r := &DataPlaneResource{client: c}

			diags := r.deleteDataPlane(context.Background(), "dp-1")

			if gotMethod != http.MethodDelete {
				t.Fatalf("method = %q, want DELETE", gotMethod)
			}
			if want := "/orgs/current/data-planes/dp-1"; gotPath != want {
				t.Fatalf("path = %q, want %q", gotPath, want)
			}
			if got := diags.HasError(); got != tc.wantErr {
				t.Fatalf("HasError() = %v, want %v (diags: %v)", got, tc.wantErr, diags)
			}
			if got := diags.WarningsCount() > 0; got != tc.wantWarning {
				t.Fatalf("warning present = %v, want %v (diags: %v)", got, tc.wantWarning, diags)
			}
		})
	}
}

// TestAccDataPlaneResource_basic provisions a real BYOC data plane. Deletion is
// attempted on destroy, but if this deployment does not expose the delete
// endpoint the data plane LEAKS and must be deprovisioned via LangSmith
// support. It is therefore strictly opt-in: set
// LANGSMITH_TEST_DATA_PLANE_ENABLED=1, LANGSMITH_TEST_DATA_PLANE_ROLE_ARN, and
// LANGSMITH_TEST_DATA_PLANE_EXTERNAL_ID on a BYOC-enabled org to enable.
func TestAccDataPlaneResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_DATA_PLANE_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_DATA_PLANE_ENABLED=1 to enable (requires a BYOC org; the created data plane cannot be deleted via the API)")
	}
	roleARN := os.Getenv("LANGSMITH_TEST_DATA_PLANE_ROLE_ARN")
	externalID := os.Getenv("LANGSMITH_TEST_DATA_PLANE_EXTERNAL_ID")
	if roleARN == "" || externalID == "" {
		t.Skip("Set LANGSMITH_TEST_DATA_PLANE_ROLE_ARN and LANGSMITH_TEST_DATA_PLANE_EXTERNAL_ID to enable")
	}
	name := fmt.Sprintf("tf-dp-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_data_plane" "test" {
  name        = %[1]q
  region      = "us-east-1"
  external_id = %[2]q
  role_arn    = %[3]q
  vpc_cidr    = "10.42.0.0/16"
}
`, name, externalID, roleARN),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_data_plane.test", "id"),
					resource.TestCheckResourceAttr("langsmith_data_plane.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_data_plane.test", "status", "requested"),
				),
			},
		},
	})
}
