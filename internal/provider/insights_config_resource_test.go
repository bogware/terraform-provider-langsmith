// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccInsightsConfigResource_basic exercises the beta insights config
// resource end-to-end. The clustering job itself does not execute as part of
// this test — only the config CRUD path is verified.
//
// Requires trace insights to be enabled for the workspace; otherwise the API
// returns 400 "Trace insights are not enabled". Set LANGSMITH_TEST_INSIGHTS_CONFIG=1
// to run against an entitled workspace.
func TestAccInsightsConfigResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_INSIGHTS_CONFIG") == "" {
		t.Skip("Set LANGSMITH_TEST_INSIGHTS_CONFIG=1 to enable (requires trace insights / run insights entitlement)")
	}
	projectName := fmt.Sprintf("tf-insights-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	configName := fmt.Sprintf("tf-cfg-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccInsightsConfigResourceConfig(projectName, configName, "test config"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_insights_config.test", "id"),
					resource.TestCheckResourceAttr("langsmith_insights_config.test", "name", configName),
					resource.TestCheckResourceAttr("langsmith_insights_config.test", "description", "test config"),
				),
			},
			// Idempotency: server-side null expansion in `config` must not surface as drift.
			{
				Config:             testAccInsightsConfigResourceConfig(projectName, configName, "test config"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config: testAccInsightsConfigResourceConfig(projectName, configName, "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_insights_config.test", "description", "updated description"),
				),
			},
		},
	})
}

func testAccInsightsConfigResourceConfig(projectName, configName, description string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_insights_config" "test" {
  session_id  = langsmith_project.test.id
  name        = %[2]q
  description = %[3]q

  config = jsonencode({
    last_n_hours = 24
    model        = "openai"
    hierarchy    = [5, 20]
  })
}
`, projectName, configName, description)
}
