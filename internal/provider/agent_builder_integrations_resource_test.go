// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAgentBuilderIntegrationsResource_basic mutates workspace-level Agent
// Builder settings, so it is opt-in. Set LANGSMITH_TEST_AGENT_BUILDER_ENABLED=1
// to enable (requires the Agent Builder feature on the workspace).
func TestAccAgentBuilderIntegrationsResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_AGENT_BUILDER_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_AGENT_BUILDER_ENABLED=1 to enable (mutates workspace Agent Builder settings)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_agent_builder_integrations" "test" {
  integrations_enabled_by_default = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_agent_builder_integrations.test", "id", "agent_builder_integrations"),
					resource.TestCheckResourceAttr("langsmith_agent_builder_integrations.test", "integrations_enabled_by_default", "true"),
					resource.TestCheckResourceAttrSet("langsmith_agent_builder_integrations.test", "integration_catalog.#"),
				),
			},
			{
				Config: `
resource "langsmith_agent_builder_integrations" "test" {
  integrations_enabled_by_default = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_agent_builder_integrations.test", "integrations_enabled_by_default", "false"),
				),
			},
		},
	})
}
