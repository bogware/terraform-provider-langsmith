// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkspaceDataSource_byID verifies the workspace data source can look
// up a workspace by ID — riding straight to the right saloon without asking around.
func TestAccWorkspaceDataSource_byID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_workspace" "test" {
  id = "6280a0b8-8bda-455c-8655-6bc7a141668d"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_workspace.test", "id"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace.test", "display_name"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace.test", "created_at"),
				),
			},
		},
	})
}
