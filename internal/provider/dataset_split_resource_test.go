// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetSplitResource_basic(t *testing.T) {
	dsName := fmt.Sprintf("tf-split-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetSplitResourceConfig(dsName, []string{"a", "b"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_dataset_split.test", "name", "train"),
					resource.TestCheckResourceAttr("langsmith_dataset_split.test", "example_ids.#", "2"),
				),
			},
			{
				Config: testAccDatasetSplitResourceConfig(dsName, []string{"a", "b", "c"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_dataset_split.test", "example_ids.#", "3"),
				),
			},
		},
	})
}

func testAccDatasetSplitResourceConfig(dsName string, exampleKeys []string) string {
	cfg := fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = %[1]q
  data_type = "kv"
}
`, dsName)
	for _, k := range exampleKeys {
		cfg += fmt.Sprintf(`
resource "langsmith_example" "ex_%[1]s" {
  dataset_id = langsmith_dataset.test.id
  inputs     = jsonencode({ q = "question %[1]s" })
  outputs    = jsonencode({ a = "answer %[1]s" })
}
`, k)
	}
	cfg += `
resource "langsmith_dataset_split" "test" {
  dataset_id = langsmith_dataset.test.id
  name       = "train"
  example_ids = [
`
	for _, k := range exampleKeys {
		cfg += fmt.Sprintf("    langsmith_example.ex_%s.id,\n", k)
	}
	cfg += `  ]
}
`
	return cfg
}
