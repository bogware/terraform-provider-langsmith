// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccTTLSettingsResource_basic checks that time-to-live settings stick
// like a brand on a longhorn. Even in Dodge City, nothing lasts forever —
// but these traces ought to hold for at least 400 days.
func TestAccTTLSettingsResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "langsmith_ttl_settings" "test" {
  longlived_ttl_days = 400
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_ttl_settings.test", "longlived_ttl_days", "400"),
					resource.TestCheckResourceAttrSet("langsmith_ttl_settings.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_ttl_settings.test", "tenant_id"),
				),
			},
		},
	})
}
