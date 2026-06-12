// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccOrgUsageDataSource_basic targets /api/v1/orgs/current/billing/usage.
// The endpoint requires billing read permission and may not be exposed on
// self-hosted or free-tier orgs, so it is opt-in via
// LANGSMITH_TEST_ORG_USAGE_ENABLED=1.
func TestAccOrgUsageDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_ORG_USAGE_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_ORG_USAGE_ENABLED=1 to enable (requires billing access on the org)")
	}
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.AddDate(0, 0, -7)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "langsmith_org_usage" "test" {
  starting_on   = %[1]q
  ending_before = %[2]q
}
`, start.Format(time.RFC3339), end.Format(time.RFC3339)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_org_usage.test", "usage.#"),
				),
			},
		},
	})
}
