// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkspaceMemberResource_basic invites a new hand to the outfit and
// makes sure they're on the roster. Even Miss Kitty had to vouch for her
// people — this test ensures workspace membership is properly recorded.
func TestAccWorkspaceMemberResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "langsmith_workspace_member" "test" {
  email   = "tf-acc-test@example.com"
  role_id = "00000000-0000-0000-0000-000000000000"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_workspace_member.test", "id"),
					resource.TestCheckResourceAttr("langsmith_workspace_member.test", "email", "tf-acc-test@example.com"),
				),
			},
		},
	})
}
