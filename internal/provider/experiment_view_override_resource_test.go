// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccExperimentViewOverrideResource_basic(t *testing.T) {
	dsName := fmt.Sprintf("tf-evo-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccExperimentViewOverrideResourceConfig(dsName, 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_experiment_view_override.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_experiment_view_override.test", "dataset_id"),
					resource.TestCheckResourceAttr("langsmith_experiment_view_override.test", "column_overrides.#", "2"),
					resource.TestCheckResourceAttr("langsmith_experiment_view_override.test", "column_overrides.0.column", "outputs.accuracy"),
					resource.TestCheckResourceAttr("langsmith_experiment_view_override.test", "column_overrides.0.precision", "2"),
				),
			},
			{
				Config: testAccExperimentViewOverrideResourceConfig(dsName, 4),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_experiment_view_override.test", "column_overrides.0.precision", "4"),
				),
			},
		},
	})
}

func testAccExperimentViewOverrideResourceConfig(dsName string, precision int) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = %[1]q
  data_type = "kv"
}

resource "langsmith_experiment_view_override" "test" {
  dataset_id = langsmith_dataset.test.id

  column_overrides = [
    {
      column         = "outputs.accuracy"
      precision      = %[2]d
      color_gradient = jsonencode([[0, "#ff0000"], [1, "#00ff00"]])
    },
    {
      column = "inputs.question"
      hide   = true
    },
  ]
}
`, dsName, precision)
}
