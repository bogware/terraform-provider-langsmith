// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccPlaygroundSettingsResource_availabilityAndOAuth covers the two field
// groups the create endpoint treats differently: oauth_* is accepted by POST,
// while the available_in_* flags exist only on PATCH and so require the
// follow-up call Create makes. If that call were dropped, this fails with
// "provider produced inconsistent result after apply".
func TestAccPlaygroundSettingsResource_availabilityAndOAuth(t *testing.T) {
	name := fmt.Sprintf("tf-acc-ps-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPlaygroundSettingsResourceConfig(name, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_playground_settings.test", "id"),
					resource.TestCheckResourceAttr("langsmith_playground_settings.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_playground_settings.test", "available_in_playground", "false"),
					resource.TestCheckResourceAttr("langsmith_playground_settings.test", "available_in_evaluators", "false"),
					resource.TestCheckResourceAttr("langsmith_playground_settings.test", "oauth_enabled", "false"),
					resource.TestCheckResourceAttr("langsmith_playground_settings.test", "oauth_client_id", "tf-acc-client"),
				),
			},
			{
				Config: testAccPlaygroundSettingsResourceConfig(name, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_playground_settings.test", "available_in_playground", "true"),
					resource.TestCheckResourceAttr("langsmith_playground_settings.test", "available_in_evaluators", "true"),
				),
			},
		},
	})
}

func testAccPlaygroundSettingsResourceConfig(name string, available bool) string {
	return fmt.Sprintf(`
resource "langsmith_playground_settings" "test" {
  name     = %[1]q
  settings = jsonencode({ temperature = 0.7, max_tokens = 1000 })

  oauth_enabled                    = false
  oauth_client_id                  = "tf-acc-client"
  oauth_token_url                  = "https://oauth2.googleapis.com/token"
  oauth_token_endpoint_auth_method = "client_secret_post"

  available_in_playground = %[2]t
  available_in_evaluators = %[2]t
}
`, name, available)
}
