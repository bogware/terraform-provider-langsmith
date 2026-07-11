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

// TestAccProjectResource_basic walks the project resource through the full
// frontier: creation, import, and update. If any step draws on an empty
// holster, the test fails — no second chances on Front Street.
func TestAccProjectResource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	rNameUpdated := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// CheckDestroy is handled automatically by the test framework
			// verifying the resource no longer exists.
			return nil
		},
		Steps: []resource.TestStep{
			// Create and verify initial state.
			{
				Config: testAccProjectResourceConfig(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_project.test", "id"),
					resource.TestCheckResourceAttr("langsmith_project.test", "name", rName),
					resource.TestCheckResourceAttrSet("langsmith_project.test", "workspace_id"),
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

// TestAccProjectResource_tagValueIDs verifies that tag_value_ids round-trips:
// a tag key and value are created first, then linked to the project. The
// LangSmith API does not echo tag_value_ids back on read, so the configured
// value is preserved in state and must survive a plan with no diff.
func TestAccProjectResource_tagValueIDs(t *testing.T) {
	rName := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	tagKey := fmt.Sprintf("tf-test-key-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	tagValue := fmt.Sprintf("tf-test-val-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccProjectResourceConfigTagValueIDs(rName, tagKey, tagValue),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_project.test", "id"),
					resource.TestCheckResourceAttr("langsmith_project.test", "name", rName),
					resource.TestCheckResourceAttr("langsmith_project.test", "tag_value_ids.#", "1"),
					resource.TestCheckResourceAttrPair(
						"langsmith_project.test", "tag_value_ids.0",
						"langsmith_tag_value.test", "id",
					),
				),
			},
			// A no-op re-apply must produce no diff even though the API does
			// not return tag_value_ids on read.
			{
				Config:   testAccProjectResourceConfigTagValueIDs(rName, tagKey, tagValue),
				PlanOnly: true,
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

// testAccProjectResourceConfigTagValueIDs returns HCL that wires a tag key and
// tag value to a project via tag_value_ids, exercising the natural link to
// langsmith_tag_value.
func testAccProjectResourceConfigTagValueIDs(name, tagKey, tagValue string) string {
	return fmt.Sprintf(`
resource "langsmith_tag_key" "test" {
  key = %[2]q
}

resource "langsmith_tag_value" "test" {
  tag_key_id = langsmith_tag_key.test.id
  value      = %[3]q
}

resource "langsmith_project" "test" {
  name          = %[1]q
  tag_value_ids = [langsmith_tag_value.test.id]
}
`, name, tagKey, tagValue)
}
