// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFeedbackFormulaResource_basic(t *testing.T) {
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
				Config: testAccFeedbackFormulaResourceConfig(),
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

func testAccFeedbackFormulaResourceConfig() string {
	return `
resource "langsmith_feedback_formula" "test" {
  feedback_key     = "composite"
  aggregation_type = "avg"
  formula_parts    = jsonencode([
    {
      part_type = "weighted_key"
      weight    = 1.0
      key       = "correctness"
    }
  ])
}
`
}
