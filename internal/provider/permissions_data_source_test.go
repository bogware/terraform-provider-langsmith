// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPermissionsDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_ORG_PERMISSIONS") == "" {
		t.Skip("Set LANGSMITH_TEST_ORG_PERMISSIONS=1 to enable (listing org permissions returns 403 for keys without org-level RBAC access)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_permissions" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_permissions.test", "permissions.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_permissions.test", "permissions.0.name"),
					resource.TestCheckResourceAttrSet("data.langsmith_permissions.test", "permissions.0.access_scope"),
				),
			},
		},
	})
}
