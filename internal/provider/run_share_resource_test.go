// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRunShareResource_basic requires an existing run UUID
// (LANGSMITH_TEST_RUN_ID) because the API does not let us create a run
// declaratively. Set it to a real, non-deleted run.
func TestAccRunShareResource_basic(t *testing.T) {
	runID := os.Getenv("LANGSMITH_TEST_RUN_ID")
	if runID == "" {
		t.Skip("Set LANGSMITH_TEST_RUN_ID to a real run UUID to enable this acceptance test")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunShareResourceConfig(runID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_run_share.test", "run_id", runID),
					resource.TestCheckResourceAttrSet("langsmith_run_share.test", "share_token"),
				),
			},
			{
				ResourceName:      "langsmith_run_share.test",
				ImportState:       true,
				ImportStateId:     runID,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccRunShareResourceConfig(runID string) string {
	return fmt.Sprintf(`
resource "langsmith_run_share" "test" {
  run_id = %[1]q
}
`, runID)
}
