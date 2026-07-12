// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRepoTagsDataSource_basic reads the prompt-repo tag catalog. The
// catalog is server-owned and shared, so we only assert the list attribute is
// set rather than on any particular tag.
func TestAccRepoTagsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "langsmith_repo_tags" "test" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_repo_tags.test", "tags.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_repo_tags.test", "workspace_id"),
				),
			},
		},
	})
}
