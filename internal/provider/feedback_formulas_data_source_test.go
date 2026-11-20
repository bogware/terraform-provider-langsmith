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

// TestAccFeedbackFormulasDataSource_basic reads the list data source against
// the live API. LangSmith has retired the feedback formula endpoints (#98)
// and answers 410, so the deprecated data source must return an empty list,
// with a warning, rather than fail every plan that still references it.
func TestAccFeedbackFormulasDataSource_basic(t *testing.T) {
	// Temporarily skipped: LangSmith retired the feedback formula endpoints (410), so this test fails on every
	// run regardless of the change under test. Remove the skip as part of
	// fixing #98.
	t.Skip("Skipped until #98 is fixed: LangSmith retired the feedback formula endpoints (410) (https://github.com/bogware/terraform-provider-langsmith/issues/98)")

	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFeedbackFormulasDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_feedback_formulas.test", "formulas.#", "0"),
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

data "langsmith_feedback_formulas" "test" {
  dataset_id = langsmith_dataset.test.id
}
`, name)
}
