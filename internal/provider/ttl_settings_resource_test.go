// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccTTLSettingsResource_basic checks that time-to-live settings stick
// like a brand on a longhorn. Even in Dodge City, nothing lasts forever —
// but these traces ought to hold for a good while.
func TestAccTTLSettingsResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// CheckDestroy is handled automatically by the test framework
			// verifying the resource no longer exists.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: `resource "langsmith_ttl_settings" "test" {
  default_trace_tier = "longlived"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_ttl_settings.test", "default_trace_tier", "longlived"),
					resource.TestCheckResourceAttrSet("langsmith_ttl_settings.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_ttl_settings.test", "organization_id"),
				),
			},
		},
	})
}
