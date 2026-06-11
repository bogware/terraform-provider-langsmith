// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorkspacesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_workspaces" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_workspaces.test", "workspaces.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspaces.test", "workspaces.0.id"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspaces.test", "workspaces.0.display_name"),
				),
			},
		},
	})
}
