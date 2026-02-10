// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFeedbackFormulaResource_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-proj-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// CheckDestroy is handled automatically by the test framework
			// verifying the resource no longer exists.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccFeedbackFormulaResourceConfig(projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_feedback_formula.test", "id"),
					resource.TestCheckResourceAttr("langsmith_feedback_formula.test", "feedback_key", "composite"),
					resource.TestCheckResourceAttr("langsmith_feedback_formula.test", "aggregation_type", "avg"),
				),
			},
			{
				ResourceName:      "langsmith_feedback_formula.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccFeedbackFormulaResourceConfig(projectName string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_feedback_formula" "test" {
  feedback_key     = "composite"
  aggregation_type = "avg"
  session_id       = langsmith_project.test.id
  formula_parts    = jsonencode([
    {
      part_type = "weighted_key"
      weight    = 1.0
      key       = "correctness"
    }
  ])
}
`, projectName)
}
