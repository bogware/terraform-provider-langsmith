// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRunRuleLogsDataSource_basic creates a run rule and reads its logs.
// Just after creation the rule has typically never been applied, so
// last_applied is null and logs is empty; the test asserts the read succeeds
// and the computed attributes are set without error.
func TestAccRunRuleLogsDataSource_basic(t *testing.T) {
	name := fmt.Sprintf("tf-acc-rrlogs-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunRuleLogsDataSourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.langsmith_run_rule_logs.test", "rule_id",
						"langsmith_run_rule.test", "id",
					),
					resource.TestCheckResourceAttrSet("data.langsmith_run_rule_logs.test", "logs.#"),
				),
			},
		},
	})
}

func testAccRunRuleLogsDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_run_rule" "test" {
  display_name  = %[1]q
  sampling_rate = 0.1
  session_id    = langsmith_project.test.id
  is_enabled    = true
}

data "langsmith_run_rule_logs" "test" {
  rule_id = langsmith_run_rule.test.id
}
`, name)
}
