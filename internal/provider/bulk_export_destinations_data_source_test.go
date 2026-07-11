// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccBulkExportDestinationsDataSource_basic lists the bulk export
// destinations in the test workspace. Creating a destination requires real S3
// credentials, so the test only asserts that the listing succeeds and that the
// computed attributes are populated -- an empty list is a valid result.
func TestAccBulkExportDestinationsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBulkExportDestinationsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_bulk_export_destinations.test", "destinations.#"),
				),
			},
		},
	})
}

func testAccBulkExportDestinationsDataSourceConfig() string {
	return `
data "langsmith_bulk_export_destinations" "test" {}
`
}
