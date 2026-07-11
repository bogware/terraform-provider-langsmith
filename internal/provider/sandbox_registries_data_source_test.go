// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSandboxRegistriesDataSource_basic reads the sandbox registry list.
// The list endpoint answers 200 with an empty list when no registries are
// configured, so this runs unguarded -- it never needs to create anything.
func TestAccSandboxRegistriesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_sandbox_registries" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_sandbox_registries.test", "registries.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_sandbox_registries.test", "workspace_id"),
				),
			},
		},
	})
}
