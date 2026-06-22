// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccMCPVendorsDataSource_basic reads the platform MCP vendor list. The
// endpoint is read-only and available on all accounts, so no env gate is
// required.
func TestAccMCPVendorsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_mcp_vendors" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_mcp_vendors.test", "vendors.#"),
					resource.TestCheckTypeSetElemNestedAttrs(
						"data.langsmith_mcp_vendors.test",
						"vendors.*",
						map[string]string{"vendor_id": "arcade"},
					),
				),
			},
		},
	})
}
