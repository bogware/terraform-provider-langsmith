// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccMCPVendorDataSource_basic requires the slug of an enabled vendor for
// the org. Set LANGSMITH_TEST_MCP_VENDOR_SLUG (e.g. "github") to enable.
func TestAccMCPVendorDataSource_basic(t *testing.T) {
	slug := os.Getenv("LANGSMITH_TEST_MCP_VENDOR_SLUG")
	if slug == "" {
		t.Skip("Set LANGSMITH_TEST_MCP_VENDOR_SLUG to a valid vendor slug to enable")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "langsmith_mcp_vendor" "test" {
  vendor_slug = "` + slug + `"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_mcp_vendor.test", "name"),
					resource.TestCheckResourceAttrSet("data.langsmith_mcp_vendor.test", "status"),
				),
			},
		},
	})
}
