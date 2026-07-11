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

func TestAccChartResource_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-proj-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	sectionTitle := fmt.Sprintf("tf-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	chartTitle := fmt.Sprintf("tf-chart-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

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
				Config: testAccChartResourceConfig(projectName, sectionTitle, chartTitle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_chart.test", "id"),
					resource.TestCheckResourceAttr("langsmith_chart.test", "title", chartTitle),
					resource.TestCheckResourceAttr("langsmith_chart.test", "chart_type", "line"),
				),
			},
			{
				ResourceName:            "langsmith_chart.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"series", "section_id"},
			},
			{
				Config: testAccChartResourceConfigKPI(projectName, sectionTitle, chartTitle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_chart.test", "id"),
					resource.TestCheckResourceAttr("langsmith_chart.test", "chart_type", "kpi"),
				),
			},
		},
	})
}

func testAccChartResourceConfig(projectName, sectionTitle, chartTitle string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_chart_section" "test" {
  title = %[2]q
}

resource "langsmith_chart" "test" {
  title      = %[3]q
  chart_type = "line"
  section_id = langsmith_chart_section.test.id
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
`, projectName, sectionTitle, chartTitle)
}

// testAccChartResourceConfigKPI updates the chart to a "kpi" chart type to
// exercise the widened CustomChartType enum (line, bar, table, kpi, top-k, pie).
func testAccChartResourceConfigKPI(projectName, sectionTitle, chartTitle string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

resource "langsmith_chart_section" "test" {
  title = %[2]q
}

resource "langsmith_chart" "test" {
  title      = %[3]q
  chart_type = "kpi"
  section_id = langsmith_chart_section.test.id
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
`, projectName, sectionTitle, chartTitle)
}
