// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccProjectResource_basic walks the project resource through the full
// frontier: creation, import, and update. If any step draws on an empty
// holster, the test fails — no second chances on Front Street.
func TestAccProjectResource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	rNameUpdated := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify initial state.
			{
				Config: testAccProjectResourceConfig(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_project.test", "id"),
					resource.TestCheckResourceAttr("langsmith_project.test", "name", rName),
					resource.TestCheckResourceAttrSet("langsmith_project.test", "tenant_id"),
					resource.TestCheckResourceAttrSet("langsmith_project.test", "start_time"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "langsmith_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update the name and add a description.
			{
				Config: testAccProjectResourceConfig(rNameUpdated, "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_project.test", "id"),
					resource.TestCheckResourceAttr("langsmith_project.test", "name", rNameUpdated),
					resource.TestCheckResourceAttr("langsmith_project.test", "description", "updated description"),
				),
			},
		},
	})
}

// testAccProjectResourceConfig returns HCL for a project resource — plain or
// with a description, depending on what the situation calls for.
func testAccProjectResourceConfig(name, description string) string {
	if description != "" {
		return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, description)
	}
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}
`, name)
}
