// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccBulkExportsDataSource_basic lists the bulk exports in the test
// workspace. Creating a bulk export requires a destination with real S3
// credentials, so the test only asserts that the listing succeeds and that the
// computed attributes are populated -- an empty list is a valid result.
func TestAccBulkExportsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBulkExportsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_bulk_exports.test", "bulk_exports.#"),
				),
			},
		},
	})
}

func testAccBulkExportsDataSourceConfig() string {
	return `
data "langsmith_bulk_exports" "test" {}
`
}
