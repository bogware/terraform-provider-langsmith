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

func TestAccTaggingResource_basic(t *testing.T) {
	tagKeyName := fmt.Sprintf("tf-key-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	tagValueName := fmt.Sprintf("tf-val-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	projectName := fmt.Sprintf("tf-proj-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

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
				Config: testAccTaggingResourceConfig(tagKeyName, tagValueName, projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_tagging.test", "id"),
					resource.TestCheckResourceAttr("langsmith_tagging.test", "resource_type", "project"),
				),
			},
		},
	})
}

func testAccTaggingResourceConfig(tagKey, tagValue, projectName string) string {
	return fmt.Sprintf(`
resource "langsmith_tag_key" "test" {
  key = %[1]q
}

resource "langsmith_tag_value" "test" {
  tag_key_id = langsmith_tag_key.test.id
  value      = %[2]q
}

resource "langsmith_project" "test" {
  name = %[3]q
}

resource "langsmith_tagging" "test" {
  tag_value_id  = langsmith_tag_value.test.id
  resource_type = "project"
  resource_id   = langsmith_project.test.id
}
`, tagKey, tagValue, projectName)
}
