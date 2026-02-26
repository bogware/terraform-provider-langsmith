// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccUserDataSource_byEmail verifies the user data source can look up a
// user by email and return the canonical user ID.
func TestAccUserDataSource_byEmail(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_user" "test" {
  email = "user@example.com"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_user.test", "id"),
					resource.TestCheckResourceAttrSet("data.langsmith_user.test", "display_name"),
				),
			},
		},
	})
}
