// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccChartPreviewDataSource_basic verifies the preview endpoint round-trips
// without error and returns a `data` attribute. Specific data point values are
// not asserted since they depend on workspace traffic.
func TestAccChartPreviewDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccChartPreviewDataSourceConfig(),
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

func testAccChartPreviewDataSourceConfig() string {
	return `
data "langsmith_chart_preview" "test" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  stride     = jsonencode({ hours = 1 })
  series = jsonencode([
    {
      name   = "Run Count"
      metric = "run_count"
    }
  ])
}
`
}

func testAccOrgChartPreviewDataSourceConfig() string {
	return `
data "langsmith_org_chart_preview" "test" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  stride     = jsonencode({ hours = 1 })
  series = jsonencode([
    {
      name   = "Run Count"
      metric = "run_count"
    }
  ])
}
`
}
