// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccProjectsDataSource_basic creates a project and verifies the list
// data source finds it when filtering by exact name.
func TestAccProjectsDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectsDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_projects.test", "projects.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_projects.test", "projects.0.name", rName),
					resource.TestCheckResourceAttrSet("data.langsmith_projects.test", "projects.0.id"),
					resource.TestCheckResourceAttrSet("data.langsmith_projects.test", "projects.0.workspace_id"),
				),
			},
		},
	})
}

func testAccProjectsDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

data "langsmith_projects" "test" {
  name = langsmith_project.test.name

  depends_on = [langsmith_project.test]
}
`, name)
}
