// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccChartSectionDataSource_byID(t *testing.T) {
	title := fmt.Sprintf("tf-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChartSectionDataSourceConfigByID(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_chart_section.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_chart_section.test", "title", title),
				),
			},
		},
	})
}

func TestAccChartSectionDataSource_byTitle(t *testing.T) {
	title := fmt.Sprintf("tf-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChartSectionDataSourceConfigByTitle(title),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_chart_section.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_chart_section.test", "title", title),
				),
			},
		},
	})
}

func testAccChartSectionDataSourceConfigByID(title string) string {
	return fmt.Sprintf(`
resource "langsmith_chart_section" "test" {
  title = %[1]q
}

data "langsmith_chart_section" "test" {
  id = langsmith_chart_section.test.id
}
`, title)
}

func testAccChartSectionDataSourceConfigByTitle(title string) string {
	return fmt.Sprintf(`
resource "langsmith_chart_section" "test" {
  title = %[1]q
}

data "langsmith_chart_section" "test" {
  title = langsmith_chart_section.test.title
}
`, title)
}
