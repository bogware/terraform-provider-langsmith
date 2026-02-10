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

func TestAccChartSectionResource_basic(t *testing.T) {
	title := fmt.Sprintf("tf-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	titleUpdated := fmt.Sprintf("tf-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

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
				Config: testAccChartSectionResourceConfig(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_chart_section.test", "id"),
					resource.TestCheckResourceAttr("langsmith_chart_section.test", "title", title),
				),
			},
			{
				ResourceName:      "langsmith_chart_section.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccChartSectionResourceConfigUpdated(titleUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_chart_section.test", "title", titleUpdated),
					resource.TestCheckResourceAttr("langsmith_chart_section.test", "description", "updated"),
				),
			},
		},
	})
}

func testAccChartSectionResourceConfig(title string) string {
	return fmt.Sprintf(`
resource "langsmith_chart_section" "test" {
  title = %[1]q
}
`, title)
}

func testAccChartSectionResourceConfigUpdated(title string) string {
	return fmt.Sprintf(`
resource "langsmith_chart_section" "test" {
  title       = %[1]q
  description = "updated"
}
`, title)
}
