// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkspaceTTLSettingsResource_basic mutates the longlived trace
// retention of a real, non-disposable workspace, so it is opt-in. Set
// LANGSMITH_TEST_TTL=1 to enable. WARNING: applying this changes the actual
// trace retention for the configured workspace.
func TestAccWorkspaceTTLSettingsResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_TTL") == "" {
		t.Skip("Set LANGSMITH_TEST_TTL=1 to enable (mutates real workspace trace retention on a non-disposable workspace)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceTTLSettingsResourceConfig(400),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_workspace_ttl_settings.test", "id"),
					resource.TestCheckResourceAttr("langsmith_workspace_ttl_settings.test", "longlived_ttl_days", "400"),
					resource.TestCheckResourceAttrSet("langsmith_workspace_ttl_settings.test", "workspace_id"),
					resource.TestCheckResourceAttrSet("langsmith_workspace_ttl_settings.test", "is_custom"),
				),
			},
			// Idempotency: replaying the same config must produce zero diff.
			{
				Config:             testAccWorkspaceTTLSettingsResourceConfig(400),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:      "langsmith_workspace_ttl_settings.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccWorkspaceTTLSettingsResourceConfig(90),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_workspace_ttl_settings.test", "longlived_ttl_days", "90"),
				),
			},
		},
	})
}

func testAccWorkspaceTTLSettingsResourceConfig(days int) string {
	return fmt.Sprintf(`
resource "langsmith_workspace_ttl_settings" "test" {
  longlived_ttl_days = %d
}
`, days)
}
