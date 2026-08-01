// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSharedTokensDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_shared_tokens" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_shared_tokens.test", "entity_count"),
					resource.TestCheckResourceAttrSet("data.langsmith_shared_tokens.test", "entities"),
					resource.TestCheckResourceAttrSet("data.langsmith_shared_tokens.test", "workspace_id"),
				),
			},
		},
	})
}
