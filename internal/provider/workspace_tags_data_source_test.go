// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkspaceTagsDataSource_basic creates a tag key with a tag value under
// it, then verifies the workspace taxonomy data source reads back a tag_keys
// list containing that key along with its nested value.
func TestAccWorkspaceTagsDataSource_basic(t *testing.T) {
	key := fmt.Sprintf("tf-wstags-key-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	value := fmt.Sprintf("tf-wstags-val-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceTagsDataSourceConfig(key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The taxonomy list must be populated. We do not assert on the
					// full contents: the workspace's taxonomy changes over time.
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_tags.test", "tag_keys.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_tags.test", "workspace_id"),
					// The key we just created must be in there. Its nested values are
					// not asserted positionally: tag_keys ordering is server-defined.
					resource.TestCheckTypeSetElemNestedAttrs("data.langsmith_workspace_tags.test", "tag_keys.*", map[string]string{
						"key":         key,
						"description": "workspace tags data source test",
					}),
				),
			},
		},
	})
}

func testAccWorkspaceTagsDataSourceConfig(key, value string) string {
	return fmt.Sprintf(`
resource "langsmith_tag_key" "test" {
  key         = %[1]q
  description = "workspace tags data source test"
}

resource "langsmith_tag_value" "test" {
  tag_key_id = langsmith_tag_key.test.id
  value      = %[2]q
}

data "langsmith_workspace_tags" "test" {
  depends_on = [langsmith_tag_value.test]
}
`, key, value)
}
