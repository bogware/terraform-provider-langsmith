// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccBulkExportDestinationDataSource_basic creates a bulk export destination
// resource and then reads it back via the data source, asserting that the
// computed fields match.
//
// Creating a destination requires real S3 credentials, so this test is gated
// behind LANGSMITH_TEST_S3_BUCKET / LANGSMITH_TEST_S3_ACCESS_KEY_ID /
// LANGSMITH_TEST_S3_SECRET_ACCESS_KEY (optional: LANGSMITH_TEST_S3_REGION).
func TestAccBulkExportDestinationDataSource_basic(t *testing.T) {
	bucket := os.Getenv("LANGSMITH_TEST_S3_BUCKET")
	accessKeyID := os.Getenv("LANGSMITH_TEST_S3_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("LANGSMITH_TEST_S3_SECRET_ACCESS_KEY")
	if bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		t.Skip("Set LANGSMITH_TEST_S3_BUCKET, LANGSMITH_TEST_S3_ACCESS_KEY_ID and LANGSMITH_TEST_S3_SECRET_ACCESS_KEY to enable")
	}
	region := os.Getenv("LANGSMITH_TEST_S3_REGION")
	if region == "" {
		region = "us-east-1"
	}

	displayName := fmt.Sprintf("tf-dest-ds-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBulkExportDestinationDataSourceConfig(displayName, bucket, region, accessKeyID, secretAccessKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.langsmith_bulk_export_destination.test", "id",
						"langsmith_bulk_export_destination.test", "id",
					),
					resource.TestCheckResourceAttr("data.langsmith_bulk_export_destination.test", "display_name", displayName),
					resource.TestCheckResourceAttr("data.langsmith_bulk_export_destination.test", "destination_type", "s3"),
					resource.TestCheckResourceAttr("data.langsmith_bulk_export_destination.test", "bucket_name", bucket),
					resource.TestCheckResourceAttr("data.langsmith_bulk_export_destination.test", "region", region),
					resource.TestCheckResourceAttrSet("data.langsmith_bulk_export_destination.test", "created_at"),
				),
			},
		},
	})
}

func testAccBulkExportDestinationDataSourceConfig(displayName, bucket, region, accessKeyID, secretAccessKey string) string {
	return fmt.Sprintf(`
resource "langsmith_bulk_export_destination" "test" {
  display_name      = %[1]q
  bucket_name       = %[2]q
  region            = %[3]q
  access_key_id     = %[4]q
  secret_access_key = %[5]q
}

data "langsmith_bulk_export_destination" "test" {
  id = langsmith_bulk_export_destination.test.id
}
`, displayName, bucket, region, accessKeyID, secretAccessKey)
}
