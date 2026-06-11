// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccBulkExportDataSource_basic reads an existing bulk export by ID.
// Creating a bulk export requires a bulk export destination backed by real
// S3 credentials, so this test is gated behind LANGSMITH_TEST_BULK_EXPORT_ID:
// set it to the UUID of an existing bulk export to enable.
func TestAccBulkExportDataSource_basic(t *testing.T) {
	exportID := os.Getenv("LANGSMITH_TEST_BULK_EXPORT_ID")
	if exportID == "" {
		t.Skip("Set LANGSMITH_TEST_BULK_EXPORT_ID to the UUID of an existing bulk export to enable")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBulkExportDataSourceConfig(exportID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_bulk_export.test", "id", exportID),
					resource.TestCheckResourceAttrSet("data.langsmith_bulk_export.test", "bulk_export_destination_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_bulk_export.test", "session_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_bulk_export.test", "status"),
					resource.TestCheckResourceAttrSet("data.langsmith_bulk_export.test", "created_at"),
				),
			},
		},
	})
}

// testAccBulkExportDataSourceConfig returns HCL that reads a bulk export by ID.
func testAccBulkExportDataSourceConfig(id string) string {
	return fmt.Sprintf(`
data "langsmith_bulk_export" "test" {
  id = %[1]q
}
`, id)
}
