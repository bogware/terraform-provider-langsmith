// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccOrganizationDataSource_basic verifies the organization data source
// returns the current org's details. Knowing who owns the ranch is half the battle.
func TestAccOrganizationDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_organization" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_organization.test", "id"),
					resource.TestCheckResourceAttrSet("data.langsmith_organization.test", "display_name"),
					resource.TestCheckResourceAttrSet("data.langsmith_organization.test", "tier"),
				),
			},
		},
	})
}
