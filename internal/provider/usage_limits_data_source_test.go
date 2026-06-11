// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccUsageLimitsDataSource_basic creates a usage limit and verifies the
// data source lists it for the current workspace.
func TestAccUsageLimitsDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_USAGE_LIMITS") == "" {
		t.Skip("Set LANGSMITH_TEST_USAGE_LIMITS=1 to enable (creating usage limits requires the organization:manage permission)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_usage_limit" "test" {
  limit_type  = "monthly_traces"
  limit_value = 1000000
}

data "langsmith_usage_limits" "test" {
  depends_on = [langsmith_usage_limit.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_usage_limits.test", "limits.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_usage_limits.test", "limits.0.limit_type"),
					resource.TestCheckResourceAttrSet("data.langsmith_usage_limits.test", "limits.0.limit_value"),
					resource.TestCheckResourceAttrSet("data.langsmith_usage_limits.test", "limits.0.workspace_id"),
				),
			},
		},
	})
}

// TestAccUsageLimitsDataSource_orgScope lists usage limits across the whole
// organization via /usage-limits/org.
func TestAccUsageLimitsDataSource_orgScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_usage_limits" "org" {
  org_scope = true
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_usage_limits.org", "limits.#"),
				),
			},
		},
	})
}
