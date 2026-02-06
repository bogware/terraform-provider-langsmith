// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAnnotationQueueResource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	rNameUpdated := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify initial state.
			{
				Config: testAccAnnotationQueueResourceConfig(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_annotation_queue.test", "id"),
					resource.TestCheckResourceAttr("langsmith_annotation_queue.test", "name", rName),
					resource.TestCheckResourceAttr("langsmith_annotation_queue.test", "enable_reservations", "true"),
					resource.TestCheckResourceAttr("langsmith_annotation_queue.test", "num_reviewers_per_item", "1"),
					resource.TestCheckResourceAttr("langsmith_annotation_queue.test", "reservation_minutes", "1"),
					resource.TestCheckResourceAttrSet("langsmith_annotation_queue.test", "tenant_id"),
					resource.TestCheckResourceAttrSet("langsmith_annotation_queue.test", "created_at"),
					resource.TestCheckResourceAttrSet("langsmith_annotation_queue.test", "updated_at"),
				),
			},
			// ImportState testing.
			{
				ResourceName:      "langsmith_annotation_queue.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update the name and add a description.
			{
				Config: testAccAnnotationQueueResourceConfig(rNameUpdated, "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_annotation_queue.test", "id"),
					resource.TestCheckResourceAttr("langsmith_annotation_queue.test", "name", rNameUpdated),
					resource.TestCheckResourceAttr("langsmith_annotation_queue.test", "description", "updated description"),
				),
			},
		},
	})
}

func testAccAnnotationQueueResourceConfig(name, description string) string {
	if description != "" {
		return fmt.Sprintf(`
resource "langsmith_annotation_queue" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, description)
	}
	return fmt.Sprintf(`
resource "langsmith_annotation_queue" "test" {
  name = %[1]q
}
`, name)
}
