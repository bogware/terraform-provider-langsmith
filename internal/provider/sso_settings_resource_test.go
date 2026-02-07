// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSSOSettingsResource_basic swings the saloon doors open with a
// SAML metadata URL and checks that single sign-on is properly posted
// on the notice board. One way in, one way out — just like the Long Branch.
func TestAccSSOSettingsResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "langsmith_sso_settings" "test" {
  metadata_url = "https://example.com/saml/metadata"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_sso_settings.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_sso_settings.test", "organization_id"),
				),
			},
		},
	})
}
