// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkspaceMembersDataSource_basic lists the workspace roster. Any
// workspace the test credentials can reach has at least one member -- the
// caller -- so members should never come back empty.
func TestAccWorkspaceMembersDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_workspace_members" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_members.test", "workspace_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_members.test", "members.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_members.test", "pending.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_members.test", "members.0.id"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_members.test", "members.0.user_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_members.test", "members.0.created_at"),
				),
			},
		},
	})
}
