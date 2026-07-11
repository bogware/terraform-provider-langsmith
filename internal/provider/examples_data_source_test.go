// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccExamplesDataSource_basic creates a dataset with a single example and
// reads it back through the list data source, both unfiltered (dataset scope
// only) and filtered by split. The config is self-contained so the test can run
// unguarded in CI.
func TestAccExamplesDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccExamplesDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Dataset-scoped read finds the single example.
					resource.TestCheckResourceAttr("data.langsmith_examples.test", "examples.#", "1"),
					resource.TestCheckResourceAttrSet("data.langsmith_examples.test", "examples.0.id"),
					resource.TestCheckResourceAttrPair(
						"data.langsmith_examples.test", "examples.0.id",
						"langsmith_example.test", "id",
					),
					resource.TestCheckResourceAttrPair(
						"data.langsmith_examples.test", "examples.0.dataset_id",
						"langsmith_dataset.test", "id",
					),
					resource.TestCheckResourceAttr("data.langsmith_examples.test", "examples.0.inputs", `{"question":"what is 2+2?"}`),
					resource.TestCheckResourceAttr("data.langsmith_examples.test", "examples.0.outputs", `{"answer":"4"}`),
					resource.TestCheckResourceAttrSet("data.langsmith_examples.test", "examples.0.name"),
					resource.TestCheckResourceAttrSet("data.langsmith_examples.test", "examples.0.created_at"),
					resource.TestCheckResourceAttrSet("data.langsmith_examples.test", "workspace_id"),

					// Matching split returns the example; a non-matching split
					// returns nothing.
					resource.TestCheckResourceAttr("data.langsmith_examples.by_split", "examples.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_examples.other_split", "examples.#", "0"),
				),
			},
		},
	})
}

func testAccExamplesDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name        = %[1]q
  description = "Acceptance test dataset for the langsmith_examples data source"
  data_type   = "kv"
}

resource "langsmith_example" "test" {
  dataset_id = langsmith_dataset.test.id
  inputs     = jsonencode({ question = "what is 2+2?" })
  outputs    = jsonencode({ answer = "4" })
  split      = "train"
}

data "langsmith_examples" "test" {
  dataset_id = langsmith_dataset.test.id

  depends_on = [langsmith_example.test]
}

data "langsmith_examples" "by_split" {
  dataset_id = langsmith_dataset.test.id
  splits     = ["train"]
  as_of      = "latest"

  depends_on = [langsmith_example.test]
}

data "langsmith_examples" "other_split" {
  dataset_id = langsmith_dataset.test.id
  splits     = ["test"]

  depends_on = [langsmith_example.test]
}
`, name)
}
