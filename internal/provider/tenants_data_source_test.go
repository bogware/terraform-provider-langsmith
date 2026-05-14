// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func testAccTenantsContainsWorkspaceID(tenantID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["data.langsmith_tenants.test"]
		if !ok {
			return fmt.Errorf("data.langsmith_tenants.test not found in state")
		}
		nStr, ok := rs.Primary.Attributes["tenants.#"]
		if !ok {
			return fmt.Errorf("tenants.# not in state")
		}
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return fmt.Errorf("parse tenants.#: %w", err)
		}
		for i := range n {
			if rs.Primary.Attributes[fmt.Sprintf("tenants.%d.id", i)] == tenantID {
				return nil
			}
		}
		return fmt.Errorf("tenant id %q not found among %d tenants", tenantID, n)
	}
}

// TestAccTenantsDataSource_basic verifies GET /api/v1/tenants returns at least
// the configured workspace and exposes nested tenant attributes.
func TestAccTenantsDataSource_basic(t *testing.T) {
	wsID := os.Getenv("LANGSMITH_TENANT_ID")
	if wsID == "" {
		t.Skip("LANGSMITH_TENANT_ID not set; skipping tenants data source acceptance test")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_tenants" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_tenants.test", "tenants.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_tenants.test", "tenants.0.id"),
					resource.TestCheckResourceAttrSet("data.langsmith_tenants.test", "tenants.0.display_name"),
					testAccTenantsContainsWorkspaceID(wsID),
				),
			},
		},
	})
}
