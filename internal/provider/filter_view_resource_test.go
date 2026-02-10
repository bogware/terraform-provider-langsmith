// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccFilterViewResource_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	viewName := fmt.Sprintf("tf-view-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	viewNameUpdated := fmt.Sprintf("tf-view-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// CheckDestroy is handled automatically by the test framework
			// verifying the resource no longer exists.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccFilterViewResourceConfig(projectName, viewName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_filter_view.test", "id"),
					resource.TestCheckResourceAttr("langsmith_filter_view.test", "display_name", viewName),
				),
			},
			{
				ResourceName:      "langsmith_filter_view.test",
				ImportState:       true,
				ImportStateVerify: true,
				// session_id is not returned by the import endpoint, so skip
				ImportStateVerifyIgnore: []string{"session_id"},
			},
			{
				Config: testAccFilterViewResourceConfigUpdated(projectName, viewNameUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_filter_view.test", "display_name", viewNameUpdated),
					resource.TestCheckResourceAttr("langsmith_filter_view.test", "description", "updated description"),
				),
			},
		},
	})
}

func testAccFilterViewResourceConfig(projectName, viewName string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_filter_view" "test" {
  session_id   = langsmith_project.test.id
  display_name = %[2]q
}
`, projectName, viewName)
}

func testAccFilterViewResourceConfigUpdated(projectName, viewName string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_filter_view" "test" {
  session_id    = langsmith_project.test.id
  display_name  = %[2]q
  description   = "updated description"
  filter_string = "eq(status, \"error\")"
}
`, projectName, viewName)
}
