// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRoleAccessPoliciesResource exercises attaching access policies to an
// organization role. The attach endpoint requires ABAC (attribute-based access
// control) to be enabled on the org; on accounts without it the API returns
// `403 ABAC is not enabled`. It is therefore gated behind LANGSMITH_TEST_ABAC,
// which must also supply a real role ID and two access policy IDs.
//
// Set:
//   - LANGSMITH_TEST_ABAC=1
//   - LANGSMITH_TEST_ROLE_ID            (the org role to attach policies to)
//   - LANGSMITH_TEST_ACCESS_POLICY_ID_1 (an existing access policy ID)
//   - LANGSMITH_TEST_ACCESS_POLICY_ID_2 (a second existing access policy ID)
func TestAccRoleAccessPoliciesResource(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_ABAC") == "" {
		t.Skip("Set LANGSMITH_TEST_ABAC=1 to enable (requires an ABAC-enabled org; the default test account returns 403 \"ABAC is not enabled\")")
	}
	roleID := os.Getenv("LANGSMITH_TEST_ROLE_ID")
	policy1 := os.Getenv("LANGSMITH_TEST_ACCESS_POLICY_ID_1")
	policy2 := os.Getenv("LANGSMITH_TEST_ACCESS_POLICY_ID_2")
	if roleID == "" || policy1 == "" || policy2 == "" {
		t.Skip("Set LANGSMITH_TEST_ROLE_ID, LANGSMITH_TEST_ACCESS_POLICY_ID_1, and LANGSMITH_TEST_ACCESS_POLICY_ID_2 to enable")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleAccessPoliciesConfig(roleID, policy1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_role_access_policies.test", "role_id", roleID),
					resource.TestCheckResourceAttr("langsmith_role_access_policies.test", "id", roleID),
					resource.TestCheckResourceAttr("langsmith_role_access_policies.test", "access_policy_ids.#", "1"),
					resource.TestCheckTypeSetElemAttr("langsmith_role_access_policies.test", "access_policy_ids.*", policy1),
				),
			},
			// Update: replace the attached set with both policies.
			{
				Config: testAccRoleAccessPoliciesConfigBoth(roleID, policy1, policy2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_role_access_policies.test", "access_policy_ids.#", "2"),
					resource.TestCheckTypeSetElemAttr("langsmith_role_access_policies.test", "access_policy_ids.*", policy1),
					resource.TestCheckTypeSetElemAttr("langsmith_role_access_policies.test", "access_policy_ids.*", policy2),
				),
			},
		},
	})
}

func testAccRoleAccessPoliciesConfig(roleID, policy string) string {
	return fmt.Sprintf(`
resource "langsmith_role_access_policies" "test" {
  role_id           = %[1]q
  access_policy_ids = [%[2]q]
}
`, roleID, policy)
}

func testAccRoleAccessPoliciesConfigBoth(roleID, policy1, policy2 string) string {
	return fmt.Sprintf(`
resource "langsmith_role_access_policies" "test" {
  role_id           = %[1]q
  access_policy_ids = [%[2]q, %[3]q]
}
`, roleID, policy1, policy2)
}
