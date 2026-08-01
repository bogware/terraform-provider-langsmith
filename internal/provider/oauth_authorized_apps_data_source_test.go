// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOAuthAuthorizedAppsDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_OAUTH") == "" {
		t.Skip("Set LANGSMITH_TEST_OAUTH=1 to enable (requires an org entitled to OAuth)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_oauth_authorized_apps" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_oauth_authorized_apps.test", "apps.#"),
				),
			},
		},
	})
}
