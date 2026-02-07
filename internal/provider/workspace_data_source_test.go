// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkspaceDataSource_byName verifies the workspace data source can look
// up a workspace by display name — like asking around Dodge City for the right saloon.
func TestAccWorkspaceDataSource_byName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_workspace" "test" {
  display_name = "default"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_workspace.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_workspace.test", "display_name", "default"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace.test", "tenant_handle"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace.test", "created_at"),
				),
			},
		},
	})
}
