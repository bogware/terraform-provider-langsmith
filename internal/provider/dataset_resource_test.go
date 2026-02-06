// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDatasetResource_basic puts the dataset resource through its paces —
// create, import, and update — like a new deputy proving his worth in Dodge.
func TestAccDatasetResource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	rNameUpdated := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify initial state.
			{
				Config: testAccDatasetResourceConfig(rName, "kv", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_dataset.test", "id"),
					resource.TestCheckResourceAttr("langsmith_dataset.test", "name", rName),
					resource.TestCheckResourceAttr("langsmith_dataset.test", "data_type", "kv"),
					resource.TestCheckResourceAttrSet("langsmith_dataset.test", "tenant_id"),
					resource.TestCheckResourceAttrSet("langsmith_dataset.test", "created_at"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "langsmith_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update the name and add a description.
			{
				Config: testAccDatasetResourceConfig(rNameUpdated, "kv", "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_dataset.test", "id"),
					resource.TestCheckResourceAttr("langsmith_dataset.test", "name", rNameUpdated),
					resource.TestCheckResourceAttr("langsmith_dataset.test", "data_type", "kv"),
					resource.TestCheckResourceAttr("langsmith_dataset.test", "description", "updated description"),
				),
			},
		},
	})
}

// testAccDatasetResourceConfig wrangles together the HCL for a dataset resource.
// Description's optional — some datasets speak for themselves, like Festus at suppertime.
func testAccDatasetResourceConfig(name, dataType, description string) string {
	if description != "" {
		return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name        = %[1]q
  data_type   = %[2]q
  description = %[3]q
}
`, name, dataType, description)
	}
	return fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = %[1]q
  data_type = %[2]q
}
`, name, dataType)
}
