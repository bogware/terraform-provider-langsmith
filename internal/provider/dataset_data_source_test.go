// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetDataSourceConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_dataset.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("data.langsmith_dataset.test", "data_type", "kv"),
					resource.TestCheckResourceAttrSet("data.langsmith_dataset.test", "tenant_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_dataset.test", "created_at"),
				),
			},
		},
	})
}

func testAccDatasetDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = %[1]q
  data_type = "kv"
}

data "langsmith_dataset" "test" {
  name = langsmith_dataset.test.name

  depends_on = [langsmith_dataset.test]
}
`, name)
}
