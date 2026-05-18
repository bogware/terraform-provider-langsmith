// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEvaluatorResource_code(t *testing.T) {
	name := fmt.Sprintf("tf-eval-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	nameUpdated := name + "-v2"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEvaluatorResourceCodeConfig(name, evaluatorCodeV1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_evaluator.test", "id"),
					resource.TestCheckResourceAttr("langsmith_evaluator.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_evaluator.test", "type", "code"),
					resource.TestCheckResourceAttr("langsmith_evaluator.test", "code_evaluator.code", evaluatorCodeV1),
					resource.TestCheckResourceAttrSet("langsmith_evaluator.test", "code_evaluator.language"),
				),
			},
			// Idempotency: replaying the same config must produce zero diff.
			{
				Config:             testAccEvaluatorResourceCodeConfig(name, evaluatorCodeV1),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:      "langsmith_evaluator.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccEvaluatorResourceCodeConfig(nameUpdated, evaluatorCodeV2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_evaluator.test", "name", nameUpdated),
					resource.TestCheckResourceAttr("langsmith_evaluator.test", "code_evaluator.code", evaluatorCodeV2),
				),
			},
		},
	})
}

const evaluatorCodeV1 = "def perform_eval(run, example):\n    return {\"score\": 1}\n"
const evaluatorCodeV2 = "def perform_eval(run, example):\n    return {\"score\": 0}\n"

func testAccEvaluatorResourceCodeConfig(name, code string) string {
	return fmt.Sprintf(`
resource "langsmith_evaluator" "test" {
  name = %[1]q
  type = "code"
  code_evaluator = {
    code     = %[2]q
    language = "python"
  }
}
`, name, code)
}
