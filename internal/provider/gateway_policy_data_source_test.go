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

func TestAccGatewayPolicyDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_GATEWAY_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_GATEWAY_ENABLED=1 to enable (requires LLM Gateway feature on the org)")
	}
	name := fmt.Sprintf("tf-gw-ds-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_gateway_policy" "test" {
  name        = %[1]q
  policy_type = "spend_cap"
  action      = "block"
  config      = jsonencode({ amount_usd = 5, window = "month" })
}

data "langsmith_gateway_policy" "test" {
  id = langsmith_gateway_policy.test.id
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_gateway_policy.test", "name", name),
				),
			},
		},
	})
}
