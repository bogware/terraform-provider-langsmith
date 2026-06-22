// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccToolsDataSource_basic creates a tool and verifies the list data
// source reads back the tools list with the created tool present.
func TestAccToolsDataSource_basic(t *testing.T) {
	handle := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolsDataSourceConfig(handle),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The list must be set and contain at least the tool we created.
					resource.TestCheckResourceAttrSet("data.langsmith_tools.test", "tools.#"),
					resource.TestCheckResourceAttrSet("data.langsmith_tools.test", "workspace_id"),
					resource.TestCheckTypeSetElemNestedAttrs("data.langsmith_tools.test", "tools.*", map[string]string{
						"handle": handle,
						"name":   "TF Acc Tool",
					}),
				),
			},
		},
	})
}

func testAccToolsDataSourceConfig(handle string) string {
	return fmt.Sprintf(`
resource "langsmith_tool" "test" {
  handle      = %[1]q
  name        = "TF Acc Tool"
  description = "Acceptance test tool for the langsmith_tools data source"
  parameters  = jsonencode({ type = "object", properties = {} })
}

data "langsmith_tools" "test" {
  depends_on = [langsmith_tool.test]
}
`, handle)
}
