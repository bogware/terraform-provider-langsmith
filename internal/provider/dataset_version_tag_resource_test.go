// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetVersionTagResource_basic(t *testing.T) {
	dsName := fmt.Sprintf("tf-vtag-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetVersionTagResourceConfig(dsName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_dataset_version_tag.test", "tag", "prod"),
					resource.TestCheckResourceAttrSet("langsmith_dataset_version_tag.test", "version_as_of"),
					resource.TestCheckResourceAttrPair(
						"langsmith_dataset_version_tag.test", "dataset_id",
						"langsmith_dataset.test", "id",
					),
				),
			},
		},
	})
}

func testAccDatasetVersionTagResourceConfig(dsName string) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = %[1]q
  data_type = "kv"
}

resource "langsmith_example" "test" {
  dataset_id = langsmith_dataset.test.id
  inputs     = jsonencode({ question = "what is up" })
  outputs    = jsonencode({ answer = "not much" })
}

resource "langsmith_dataset_version_tag" "test" {
  dataset_id = langsmith_dataset.test.id
  tag        = "prod"
  # Tag the most recent dataset version (created by the example above).
  as_of = "latest"

  depends_on = [langsmith_example.test]
}
`, dsName)
}
