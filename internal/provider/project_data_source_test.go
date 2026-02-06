// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccProjectDataSource_basic verifies we can look up a project by name.
// Even Marshal Dillon checks the wanted posters before riding out.
func TestAccProjectDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_project.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_project.test", "name", rName),
					resource.TestCheckResourceAttrSet("data.langsmith_project.test", "tenant_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_project.test", "start_time"),
				),
			},
		},
	})
}

// testAccProjectDataSourceConfig returns HCL that creates a project and then
// looks it up by name — trust, but verify, as Doc Adams would say.
func testAccProjectDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

data "langsmith_project" "test" {
  name = langsmith_project.test.name

  depends_on = [langsmith_project.test]
}
`, name)
}
