// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccMCPVendorSettingsResource_basic requires an MCP vendor registered on
// the workspace. Set LANGSMITH_TEST_MCP_VENDOR_SLUG to its slug to enable.
func TestAccMCPVendorSettingsResource_basic(t *testing.T) {
	slug := os.Getenv("LANGSMITH_TEST_MCP_VENDOR_SLUG")
	if slug == "" {
		t.Skip("Set LANGSMITH_TEST_MCP_VENDOR_SLUG to an MCP vendor slug to enable (requires the MCP vendor on the workspace)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_mcp_vendor_settings" "test" {
  vendor_slug     = %[1]q
  organization_id = "tf-acc-org"
  project_id      = "tf-acc-project"
}
`, slug),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_mcp_vendor_settings.test", "id", slug),
					resource.TestCheckResourceAttr("langsmith_mcp_vendor_settings.test", "vendor_slug", slug),
					resource.TestCheckResourceAttr("langsmith_mcp_vendor_settings.test", "organization_id", "tf-acc-org"),
					resource.TestCheckResourceAttrSet("langsmith_mcp_vendor_settings.test", "is_configured"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "langsmith_mcp_vendor_settings" "test" {
  vendor_slug     = %[1]q
  organization_id = "tf-acc-org-2"
  project_id      = "tf-acc-project-2"
}
`, slug),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_mcp_vendor_settings.test", "organization_id", "tf-acc-org-2"),
					resource.TestCheckResourceAttr("langsmith_mcp_vendor_settings.test", "project_id", "tf-acc-project-2"),
				),
			},
		},
	})
}
