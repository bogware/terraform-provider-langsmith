// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccOrganizationSettingsResource_adopt applies the resource with no
// attributes configured. This adopts the current organization without sending
// any PATCH requests, so it is safe to run against a real org; the destroy is
// state-only by design.
func TestAccOrganizationSettingsResource_adopt(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_organization_settings" "test" {
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_organization_settings.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_organization_settings.test", "is_personal"),
					resource.TestCheckResourceAttrSet("langsmith_organization_settings.test", "display_name"),
				),
			},
		},
	})
}
