// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccToolResource_basic(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-tool-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccToolResourceConfig(handle, "Lookup customer"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_tool.test", "handle", handle),
					resource.TestCheckResourceAttr("langsmith_tool.test", "name", "Lookup customer"),
					resource.TestCheckResourceAttrSet("langsmith_tool.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_tool.test", "tenant_id"),
				),
			},
			{
				ResourceName:      "langsmith_tool.test",
				ImportState:       true,
				ImportStateId:     handle,
				ImportStateVerify: true,
			},
			{
				Config: testAccToolResourceConfig(handle, "Lookup customer v2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_tool.test", "name", "Lookup customer v2"),
				),
			},
		},
	})
}

func testAccToolResourceConfig(handle, name string) string {
	return fmt.Sprintf(`
resource "langsmith_tool" "test" {
  handle      = %[1]q
  name        = %[2]q
  description = "Acceptance-test tool"
  parameters = jsonencode({
    type = "object"
    properties = {
      email = { type = "string" }
    }
    required = ["email"]
  })
}
`, handle, name)
}
