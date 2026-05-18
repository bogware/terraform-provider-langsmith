// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetShareResource_basic(t *testing.T) {
	dsName := fmt.Sprintf("tf-share-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetShareResourceConfig(dsName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_dataset.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_dataset_share.test", "share_token"),
					resource.TestCheckResourceAttrPair(
						"langsmith_dataset_share.test", "dataset_id",
						"langsmith_dataset.test", "id",
					),
				),
			},
		},
	})
}

func testAccDatasetShareResourceConfig(dsName string) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = %[1]q
  data_type = "kv"
}

resource "langsmith_dataset_share" "test" {
  dataset_id = langsmith_dataset.test.id
}
`, dsName)
}
