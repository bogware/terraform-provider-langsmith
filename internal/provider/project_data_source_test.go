// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

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
