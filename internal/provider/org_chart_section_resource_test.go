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

func TestAccOrgChartSectionResource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
	title := fmt.Sprintf("tf-org-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	titleUpdated := fmt.Sprintf("tf-org-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccOrgChartSectionResourceConfig(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_org_chart_section.test", "id"),
					resource.TestCheckResourceAttr("langsmith_org_chart_section.test", "title", title),
				),
			},
			{
				ResourceName:      "langsmith_org_chart_section.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccOrgChartSectionResourceConfigUpdated(titleUpdated),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_org_chart_section.test", "title", titleUpdated),
					resource.TestCheckResourceAttr("langsmith_org_chart_section.test", "description", "updated"),
				),
			},
		},
	})
}

func testAccOrgChartSectionResourceConfig(title string) string {
	return fmt.Sprintf(`
resource "langsmith_org_chart_section" "test" {
  title = %[1]q
}
`, title)
}

func testAccOrgChartSectionResourceConfigUpdated(title string) string {
	return fmt.Sprintf(`
resource "langsmith_org_chart_section" "test" {
  title       = %[1]q
  description = "updated"
}
`, title)
}
