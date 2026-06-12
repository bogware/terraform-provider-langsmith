// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccHubEnvironmentsDataSource_basic creates a hub environment record and
// reads it back through the data source. The /api/v1/hub/environments
// endpoint is in the LangSmith OpenAPI spec but returns 404 on the hosted API
// (likely still being rolled out), so this is gated behind
// LANGSMITH_TEST_HUB_ENVIRONMENTS, matching TestAccHubEnvironmentResource_basic.
func TestAccHubEnvironmentsDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_HUB_ENVIRONMENTS") == "" {
		t.Skip("Set LANGSMITH_TEST_HUB_ENVIRONMENTS=1 to enable (endpoint may not be deployed yet on the public LangSmith API)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_hub_environment" "test" {
  environments = [
    { name = "tf-staging" },
    { name = "tf-production" },
  ]
}

data "langsmith_hub_environments" "test" {
  depends_on = [langsmith_hub_environment.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_hub_environments.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_hub_environments.test", "environments.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_hub_environments.test", "environments.0.name", "tf-staging"),
					resource.TestCheckResourceAttr("data.langsmith_hub_environments.test", "environments.1.name", "tf-production"),
				),
			},
		},
	})
}
