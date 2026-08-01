// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccMCPVendorDetailsDataSource_basic needs a vendor slug that exists in the
// workspace; the per-vendor endpoints 404 for anything else.
func TestAccMCPVendorDetailsDataSource_basic(t *testing.T) {
	slug := os.Getenv("LANGSMITH_TEST_MCP_VENDOR_SLUG")
	if slug == "" {
		t.Skip("Set LANGSMITH_TEST_MCP_VENDOR_SLUG to the slug of a configured MCP vendor to enable")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "langsmith_mcp_vendor_details" "test" {
  vendor_slug   = %[1]q
  include_tools = true
}
`, slug),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_mcp_vendor_details.test", "vendor_slug", slug),
					resource.TestCheckResourceAttrSet("data.langsmith_mcp_vendor_details.test", "tools"),
				),
			},
		},
	})
}
