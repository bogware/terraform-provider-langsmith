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

func TestAccToolDataSource_basic(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-tool-ds-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_tool" "test" {
  handle      = %[1]q
  name        = "tf-ds tool"
  description = "ds test"
  parameters  = jsonencode({ type = "object", properties = {} })
}

data "langsmith_tool" "test" {
  handle = langsmith_tool.test.handle
}
`, handle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "handle", handle),
					resource.TestCheckResourceAttr("data.langsmith_tool.test", "name", "tf-ds tool"),
				),
			},
		},
	})
}
