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

// TestAccGatewayPolicyResource_basic requires the LLM Gateway feature to be
// enabled on the org. Set LANGSMITH_TEST_GATEWAY_ENABLED=1 to enable.
func TestAccGatewayPolicyResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_GATEWAY_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_GATEWAY_ENABLED=1 to enable (requires LLM Gateway feature on the org)")
	}
	name := fmt.Sprintf("tf-gw-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_gateway_policy" "test" {
  name        = %[1]q
  description = "tf acceptance test"
  policy_type = "spend_cap"
  action      = "block"
  enabled     = true
  priority    = 50
  config      = jsonencode({ amount_usd = 10, window = "month" })
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_gateway_policy.test", "id"),
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "policy_type", "spend_cap"),
				),
			},
		},
	})
}
