// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDatasetsDataSource_basic creates a dataset and verifies the list
// data source finds it when filtering by exact name.
func TestAccDatasetsDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetsDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_datasets.test", "datasets.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.test", "datasets.0.name", rName),
					resource.TestCheckResourceAttr("data.langsmith_datasets.test", "datasets.0.data_type", "kv"),
					resource.TestCheckResourceAttrSet("data.langsmith_datasets.test", "datasets.0.id"),
					resource.TestCheckResourceAttrSet("data.langsmith_datasets.test", "datasets.0.workspace_id"),
				),
			},
		},
	})
}

func testAccDatasetsDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name        = %[1]q
  description = "Acceptance test dataset for the langsmith_datasets data source"
  data_type   = "kv"
}

data "langsmith_datasets" "test" {
  name = langsmith_dataset.test.name

  depends_on = [langsmith_dataset.test]
}
`, name)
}
