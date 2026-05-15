// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEvaluatorDataSource_basic(t *testing.T) {
	name := fmt.Sprintf("tf-eval-ds-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_evaluator" "test" {
  name = %[1]q
  type = "code"
  code_evaluator = {
    code = "def perform_eval(run, example):\n    return {\"score\": 1}\n"
  }
}

data "langsmith_evaluator" "test" {
  id = langsmith_evaluator.test.id
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_evaluator.test", "name", name),
					resource.TestCheckResourceAttr("data.langsmith_evaluator.test", "type", "code"),
					resource.TestCheckResourceAttrSet("data.langsmith_evaluator.test", "code_evaluator_json"),
				),
			},
		},
	})
}
