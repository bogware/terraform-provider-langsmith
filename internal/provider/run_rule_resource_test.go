// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRunRuleResource_retentionFlags exercises the trace-retention and
// tracing attributes. They are Optional+Computed and refreshed from the API, so
// a mismatch between what is sent and what comes back surfaces here as
// "provider produced inconsistent result after apply".
//
// The retention flags require an evaluator on the rule ("Trace retention
// extension requires an llm as a judge evaluator or code evaluator", HTTP 422),
// which is why the config below carries a code evaluator.
//
// The code evaluator does double duty as a regression test for two defects the
// second step would otherwise hit: the API assigns an evaluator_id on create,
// which must not then be sent back alongside code_evaluators ("Provide either
// evaluator_id or evaluators/code_evaluators, not both", HTTP 422), and it
// expands the evaluator with language: "python", which must not overwrite the
// configured JSON and leave a permanent diff.
func TestAccRunRuleResource_retentionFlags(t *testing.T) {
	name := fmt.Sprintf("tf-acc-rr-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunRuleResourceConfig(name, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_run_rule.test", "id"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "display_name", name),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "is_tracing_disabled", "true"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_evaluator_trace_retention", "true"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_dataset_trace_retention", "true"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_annotation_queue_trace_retention", "true"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_webhook_trace_retention", "true"),
				),
			},
			// Flip every flag: the update endpoint accepts all of them, so this
			// must apply in place rather than force a replacement.
			{
				Config: testAccRunRuleResourceConfig(name, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "is_tracing_disabled", "false"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_evaluator_trace_retention", "false"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_dataset_trace_retention", "false"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_annotation_queue_trace_retention", "false"),
					resource.TestCheckResourceAttr("langsmith_run_rule.test", "extend_webhook_trace_retention", "false"),
				),
			},
		},
	})
}

// testAccRunRuleResourceConfig carries a code evaluator because the API rejects
// any trace-retention extension without one: "Trace retention extension requires
// an llm as a judge evaluator or code evaluator" (HTTP 422).
func testAccRunRuleResourceConfig(name string, flags bool) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_run_rule" "test" {
  display_name  = %[1]q
  session_id    = langsmith_project.test.id
  sampling_rate = 1

  # language is deliberately omitted: the API fills it in, and Read must keep
  # the configured form rather than the server's expanded copy.
  code_evaluators = jsonencode([
    {
      code = "def perform_eval(run):\n    return {\"score\": 1}\n"
    }
  ])

  is_tracing_disabled                     = %[2]t
  extend_evaluator_trace_retention        = %[2]t
  extend_dataset_trace_retention          = %[2]t
  extend_annotation_queue_trace_retention = %[2]t
  extend_webhook_trace_retention          = %[2]t
}
`, name, flags)
}
