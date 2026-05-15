// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrgChartDataSource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
	projectName := fmt.Sprintf("tf-proj-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	sectionTitle := fmt.Sprintf("tf-org-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	chartTitle := fmt.Sprintf("tf-org-chart-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrgChartDataSourceConfig(projectName, sectionTitle, chartTitle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_org_chart.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_org_chart.test", "title", chartTitle),
					resource.TestCheckResourceAttr("data.langsmith_org_chart.test", "chart_type", "line"),
				),
			},
		},
	})
}

func testAccOrgChartDataSourceConfig(projectName, sectionTitle, chartTitle string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_org_chart_section" "test" {
  title = %[2]q
}

resource "langsmith_org_chart" "test" {
  title      = %[3]q
  chart_type = "line"
  section_id = langsmith_org_chart_section.test.id
  series     = jsonencode([
    {
      name   = "Run Count"
      metric = "run_count"
      filters = {
        session = [langsmith_project.test.id]
      }
    }
  ])
}

data "langsmith_org_chart" "test" {
  id = langsmith_org_chart.test.id
}
`, projectName, sectionTitle, chartTitle)
}
