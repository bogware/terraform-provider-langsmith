// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccOrgMembersDataSource_basic lists the organization roster. The
// organization always contains at least the calling identity.
func TestAccOrgMembersDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_org_members" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_org_members.test", "organization_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_org_members.test", "members.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_org_members.test", "pending.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_org_members.test", "members.0.id"),
					resource.TestCheckResourceAttrSet("data.langsmith_org_members.test", "members.0.user_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_org_members.test", "members.0.created_at"),
				),
			},
		},
	})
}
