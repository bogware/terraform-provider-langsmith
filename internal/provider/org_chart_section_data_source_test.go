// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrgChartSectionDataSource_byID(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
	title := fmt.Sprintf("tf-org-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrgChartSectionDataSourceConfigByID(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_org_chart_section.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_org_chart_section.test", "title", title),
				),
			},
		},
	})
}

func TestAccOrgChartSectionDataSource_byTitle(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
	title := fmt.Sprintf("tf-org-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrgChartSectionDataSourceConfigByTitle(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_org_chart_section.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_org_chart_section.test", "title", title),
				),
			},
		},
	})
}

func testAccOrgChartSectionDataSourceConfigByID(title string) string {
	return fmt.Sprintf(`
resource "langsmith_org_chart_section" "test" {
  title = %[1]q
}

data "langsmith_org_chart_section" "test" {
  id = langsmith_org_chart_section.test.id
}
`, title)
}

func testAccOrgChartSectionDataSourceConfigByTitle(title string) string {
	return fmt.Sprintf(`
resource "langsmith_org_chart_section" "test" {
  title = %[1]q
}

data "langsmith_org_chart_section" "test" {
  title = langsmith_org_chart_section.test.title
}
`, title)
}
