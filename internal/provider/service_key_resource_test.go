// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccServiceKeyResource_basic creates an org-wide service key. The API only
// permits this for organization admins ("Only org admins can create org-wide
// keys"), so it is gated behind an env var like other org-admin-only tests.
func TestAccServiceKeyResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_SERVICE_KEYS_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_SERVICE_KEYS_ENABLED to enable (creating org-wide service keys requires organization admin permissions)")
	}

	description := fmt.Sprintf("tf-acc-service-key-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceKeyResourceConfig(description),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_service_key.test", "id"),
					resource.TestCheckResourceAttr("langsmith_service_key.test", "description", description),
					resource.TestCheckResourceAttr("langsmith_service_key.test", "read_only", "false"),
					resource.TestCheckResourceAttrSet("langsmith_service_key.test", "key"),
					resource.TestCheckResourceAttrSet("langsmith_service_key.test", "short_key"),
				),
			},
		},
	})
}

// TestAccServiceKeyResource_roleUpdate exercises an in-place role change via
// PATCH. It requires organization admin permissions (ORGANIZATION_MANAGE) and
// two valid workspace-level role UUIDs, so it is gated behind env vars.
func TestAccServiceKeyResource_roleUpdate(t *testing.T) {
	roleID := os.Getenv("LANGSMITH_TEST_ROLE_ID")
	roleIDUpdated := os.Getenv("LANGSMITH_TEST_ROLE_ID_2")
	if roleID == "" || roleIDUpdated == "" {
		t.Skip("Set LANGSMITH_TEST_ROLE_ID and LANGSMITH_TEST_ROLE_ID_2 to two workspace role UUIDs to enable (requires organization admin permissions)")
	}

	description := fmt.Sprintf("tf-acc-service-key-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceKeyResourceRoleConfig(description, roleID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_service_key.test", "id"),
					resource.TestCheckResourceAttr("langsmith_service_key.test", "role_id", roleID),
				),
			},
			// In-place role change must update without forcing recreation.
			{
				Config: testAccServiceKeyResourceRoleConfig(description, roleIDUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_service_key.test", "role_id", roleIDUpdated),
				),
			},
		},
	})
}

func testAccServiceKeyResourceConfig(description string) string {
	return fmt.Sprintf(`
resource "langsmith_service_key" "test" {
  description = %[1]q
  read_only   = false
}
`, description)
}

func testAccServiceKeyResourceRoleConfig(description, roleID string) string {
	return fmt.Sprintf(`
resource "langsmith_service_key" "test" {
  description = %[1]q
  read_only   = false
  role_id     = %[2]q
}
`, description, roleID)
}
