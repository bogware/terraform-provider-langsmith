// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAlertRuleResource_basic sets up a lookout on the project and waits
// for trouble. Marshal Dillon never let a disturbance go unnoticed, and
// neither should your alert rules — if latency crosses the line, you'll know.
func TestAccAlertRuleResource_basic(t *testing.T) {
	rName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = "tf-acc-test-alert-%s"
}

resource "langsmith_alert_rule" "test" {
  session_id     = langsmith_project.test.id
  name           = "tf-acc-test-alert-%s"
  description    = "Test alert rule"
  type           = "threshold"
  aggregation    = "avg"
  attribute      = "latency"
  operator       = "gte"
  window_minutes = 5
  threshold      = 5000
  actions        = "[]"
}`, rName, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_alert_rule.test", "id"),
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "name", fmt.Sprintf("tf-acc-test-alert-%s", rName)),
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "type", "threshold"),
				),
			},
		},
	})
}
