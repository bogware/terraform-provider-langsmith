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

func TestAccOrgChartResource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
	projectName := fmt.Sprintf("tf-proj-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	sectionTitle := fmt.Sprintf("tf-org-section-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	chartTitle := fmt.Sprintf("tf-org-chart-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccOrgChartResourceConfig(projectName, sectionTitle, chartTitle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_org_chart.test", "id"),
					resource.TestCheckResourceAttr("langsmith_org_chart.test", "title", chartTitle),
					resource.TestCheckResourceAttr("langsmith_org_chart.test", "chart_type", "line"),
				),
			},
			{
				ResourceName:            "langsmith_org_chart.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"series", "section_id"},
			},
		},
	})
}

func testAccOrgChartResourceConfig(projectName, sectionTitle, chartTitle string) string {
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
`, projectName, sectionTitle, chartTitle)
}
