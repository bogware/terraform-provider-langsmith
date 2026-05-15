// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccHubEnvironmentResource_basic exercises /api/v1/hub/environments. As
// of writing, that endpoint is in the LangSmith OpenAPI but returns 404 on
// the hosted API — likely still being rolled out. Set
// LANGSMITH_TEST_HUB_ENVIRONMENTS=1 to attempt anyway.
func TestAccHubEnvironmentResource_basic(t *testing.T) {
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
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_hub_environment.test", "id"),
					resource.TestCheckResourceAttr("langsmith_hub_environment.test", "environments.#", "2"),
					resource.TestCheckResourceAttr("langsmith_hub_environment.test", "environments.0.name", "tf-staging"),
					resource.TestCheckResourceAttr("langsmith_hub_environment.test", "environments.1.name", "tf-production"),
				),
			},
			{
				ResourceName:      "langsmith_hub_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: `
resource "langsmith_hub_environment" "test" {
  environments = [
    { name = "tf-staging" },
    { name = "tf-production" },
    { name = "tf-canary" },
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_hub_environment.test", "environments.#", "3"),
				),
			},
		},
	})
}
