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

// TestAccGatewayPoliciesDataSource_basic creates a gateway policy and verifies
// both the unfiltered list and the server-side policy_type filter return it.
//
// The endpoint requires the LLM Gateway feature to be enabled on the
// organization; without it the API returns 403 "LLM Gateway not enabled for
// this organization". Set LANGSMITH_TEST_GATEWAY_ENABLED=1 to run this against
// a gateway-enabled org.
func TestAccGatewayPoliciesDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_GATEWAY_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_GATEWAY_ENABLED=1 to enable (requires LLM Gateway feature on the org)")
	}
	name := fmt.Sprintf("tf-gw-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGatewayPoliciesDataSourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_gateway_policies.all", "gateway_policies.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_gateway_policies.all", "workspace_id"),
					resource.TestCheckTypeSetElemNestedAttrs("data.langsmith_gateway_policies.all", "gateway_policies.*", map[string]string{
						"name":        name,
						"policy_type": "spend_cap",
						"action":      "block",
						"enabled":     "true",
					}),
					// The server-side policy_type filter must still return it.
					resource.TestCheckTypeSetElemNestedAttrs("data.langsmith_gateway_policies.spend_caps", "gateway_policies.*", map[string]string{
						"name": name,
					}),
				),
			},
		},
	})
}

func testAccGatewayPoliciesDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_gateway_policy" "test" {
  name        = %[1]q
  description = "Acceptance test policy for the langsmith_gateway_policies data source"
  policy_type = "spend_cap"
  action      = "block"
  enabled     = true
  priority    = 50
  config      = jsonencode({ amount_usd = 10, window = "month" })
}

data "langsmith_gateway_policies" "all" {
  depends_on = [langsmith_gateway_policy.test]
}

data "langsmith_gateway_policies" "spend_caps" {
  policy_type = "spend_cap"
  depends_on  = [langsmith_gateway_policy.test]
}
`, name)
}
