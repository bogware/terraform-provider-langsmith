// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetVersionsDataSource_basic(t *testing.T) {
	name := fmt.Sprintf("tf-acc-dsv-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name        = %[1]q
  description = "dataset versions data source test"
}

data "langsmith_dataset_versions" "test" {
  dataset_id = langsmith_dataset.test.id
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_dataset_versions.test", "versions.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_dataset_versions.test", "workspace_id"),
				),
			},
		},
	})
}
