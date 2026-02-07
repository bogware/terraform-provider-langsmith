// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccOrgRoleResource_basic pins a badge on a new role and makes sure
// it carries the right authority. In Dodge City every deputy needs clear
// jurisdiction — same goes for organization roles in LangSmith.
func TestAccOrgRoleResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "langsmith_org_role" "test" {
  display_name = "tf-acc-test-role"
  description  = "Test role for acceptance testing"
  permissions  = "[\"workspace:read\"]"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_org_role.test", "id"),
					resource.TestCheckResourceAttr("langsmith_org_role.test", "display_name", "tf-acc-test-role"),
				),
			},
		},
	})
}
