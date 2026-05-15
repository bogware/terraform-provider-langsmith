// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccChartPreviewDataSource_basic verifies the preview endpoint round-trips
// without error and returns a `data` attribute. Specific data point values are
// not asserted since they depend on workspace traffic. Workspace-scoped previews
// require a session (project) filter, so we create a throwaway project.
func TestAccChartPreviewDataSource_basic(t *testing.T) {
	projectName := fmt.Sprintf("tf-proj-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChartPreviewDataSourceConfig(projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_chart_preview.test", "data"),
				),
			},
		},
	})
}

func TestAccOrgChartPreviewDataSource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrgChartPreviewDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_org_chart_preview.test", "data"),
				),
			},
		},
	})
}

func testAccChartPreviewDataSourceConfig(projectName string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = %[1]q
}

data "langsmith_chart_preview" "test" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  stride     = jsonencode({ hours = 1 })
  series = jsonencode([
    {
      id     = "00000000-0000-0000-0000-000000000001"
      name   = "Run Count"
      metric = "run_count"
      filters = {
        session = [langsmith_project.test.id]
      }
    }
  ])
}
`, projectName)
}

func testAccOrgChartPreviewDataSourceConfig() string {
	return `
data "langsmith_org_chart_preview" "test" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  stride     = jsonencode({ hours = 1 })
  series = jsonencode([
    {
      id     = "00000000-0000-0000-0000-000000000001"
      name   = "Run Count"
      metric = "run_count"
    }
  ])
}
`
}
