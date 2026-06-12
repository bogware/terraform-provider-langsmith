// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccExampleDataSource_basic creates a dataset with one example and then
// looks the example up by ID through the data source.
func TestAccExampleDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccExampleDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_example.test", "id"),
					resource.TestCheckResourceAttrPair("data.langsmith_example.test", "id", "langsmith_example.test", "id"),
					resource.TestCheckResourceAttrPair("data.langsmith_example.test", "dataset_id", "langsmith_dataset.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_example.test", "inputs", `{"question":"What is 2+2?"}`),
					resource.TestCheckResourceAttr("data.langsmith_example.test", "outputs", `{"answer":"4"}`),
					resource.TestCheckResourceAttrSet("data.langsmith_example.test", "created_at"),
				),
			},
		},
	})
}

// testAccExampleDataSourceConfig returns HCL that creates a dataset and an
// example, then reads the example back through the data source.
func testAccExampleDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = %[1]q
  data_type = "kv"
}

resource "langsmith_example" "test" {
  dataset_id = langsmith_dataset.test.id
  inputs     = jsonencode({ question = "What is 2+2?" })
  outputs    = jsonencode({ answer = "4" })
}

data "langsmith_example" "test" {
  id = langsmith_example.test.id
}
`, name)
}
