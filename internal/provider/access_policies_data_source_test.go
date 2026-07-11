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

// TestAccAccessPoliciesDataSource_basic creates an access policy and verifies
// the list data source reads back the org's policies with the created one
// present.
//
// The endpoint requires ABAC (attribute-based access control) to be enabled on
// the organization, an enterprise-tier feature; without it the API returns
// 403 "ABAC is not enabled for this organization". Set LANGSMITH_TEST_ABAC=1 to
// run this against an ABAC-enabled org.
func TestAccAccessPoliciesDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_ABAC") == "" {
		t.Skip("Set LANGSMITH_TEST_ABAC=1 to enable (requires ABAC enabled on the organization)")
	}
	name := fmt.Sprintf("tf-ap-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAccessPoliciesDataSourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_access_policies.test", "access_policies.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_access_policies.test", "workspace_id"),
					resource.TestCheckTypeSetElemNestedAttrs("data.langsmith_access_policies.test", "access_policies.*", map[string]string{
						"name":   name,
						"effect": "allow",
					}),
				),
			},
		},
	})
}

func testAccAccessPoliciesDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_access_policy" "test" {
  name        = %[1]q
  description = "Acceptance test policy for the langsmith_access_policies data source"
  effect      = "allow"
}

data "langsmith_access_policies" "test" {
  depends_on = [langsmith_access_policy.test]
}
`, name)
}
