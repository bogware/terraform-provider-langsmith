// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccAuditLogDataSource_basic requires audit logs to be enabled for the
// org. Set LANGSMITH_TEST_AUDIT_LOGS_ENABLED=1 to attempt.
func TestAccAuditLogDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_AUDIT_LOGS_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_AUDIT_LOGS_ENABLED=1 to enable (requires audit-log entitlement)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "langsmith_audit_log" "test" {
  start_time = "2024-01-01T00:00:00Z"
  end_time   = "2030-01-01T00:00:00Z"
  limit      = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_audit_log.test", "items.#"),
				),
			},
		},
	})
}
