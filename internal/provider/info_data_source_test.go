// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccInfoDataSource_basic checks that the server info endpoint answers
// when called upon. In Dodge City, a man's got a right to know who he's dealing with.
func TestAccInfoDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccInfoDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_info.test", "id", "info"),
					resource.TestCheckResourceAttrSet("data.langsmith_info.test", "version"),
				),
			},
		},
	})
}

// testAccInfoDataSourceConfig returns the simplest HCL there is — just asking
// the server to state its name and business. Nothing more, nothing less.
func testAccInfoDataSourceConfig() string {
	return `
data "langsmith_info" "test" {}
`
}
