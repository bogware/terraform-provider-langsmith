// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFeedbackFormulasDataSource_basic creates a dataset and a
// dataset-scoped feedback formula, then reads the formula back through the list
// data source. The config is self-contained so the test can run unguarded in CI.
func TestAccFeedbackFormulasDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFeedbackFormulasDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_feedback_formulas.test", "formulas.#", "1"),
					resource.TestCheckResourceAttrPair(
						"data.langsmith_feedback_formulas.test", "formulas.0.id",
						"langsmith_feedback_formula.test", "id",
					),
					resource.TestCheckResourceAttrPair(
						"data.langsmith_feedback_formulas.test", "formulas.0.dataset_id",
						"langsmith_dataset.test", "id",
					),
					resource.TestCheckResourceAttr("data.langsmith_feedback_formulas.test", "formulas.0.feedback_key", "composite"),
					resource.TestCheckResourceAttr("data.langsmith_feedback_formulas.test", "formulas.0.aggregation_type", "avg"),
					resource.TestCheckResourceAttrSet("data.langsmith_feedback_formulas.test", "formulas.0.formula_parts"),
					resource.TestCheckResourceAttrSet("data.langsmith_feedback_formulas.test", "formulas.0.created_at"),
					resource.TestCheckResourceAttrSet("data.langsmith_feedback_formulas.test", "workspace_id"),
				),
			},
		},
	})
}

// TestAccFeedbackFormulasDataSource_scopeValidation asserts that the
// ExactlyOneOf config validator rejects both an unscoped read and a read that
// sets dataset_id and session_id together, so users get a plan-time error
// rather than the API's HTTP 400.
func TestAccFeedbackFormulasDataSource_scopeValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "langsmith_feedback_formulas" "none" {}
`,
				// With neither scope set the framework reports a missing
				// attribute; with both set it reports an invalid combination.
				ExpectError: regexp.MustCompile(`Missing Attribute Configuration`),
			},
			{
				Config: `
data "langsmith_feedback_formulas" "both" {
  dataset_id = "3b1c8d0e-0000-4000-8000-000000000001"
  session_id = "3b1c8d0e-0000-4000-8000-000000000002"
}
`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}

func testAccFeedbackFormulasDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name        = %[1]q
  description = "Acceptance test dataset for the langsmith_feedback_formulas data source"
  data_type   = "kv"
}

resource "langsmith_feedback_formula" "test" {
  feedback_key     = "composite"
  aggregation_type = "avg"
  dataset_id       = langsmith_dataset.test.id
  formula_parts = jsonencode([
    {
      part_type = "weighted_key"
      weight    = 1.0
      key       = "correctness"
    }
  ])
}

data "langsmith_feedback_formulas" "test" {
  dataset_id = langsmith_dataset.test.id

  depends_on = [langsmith_feedback_formula.test]
}
`, name)
}
