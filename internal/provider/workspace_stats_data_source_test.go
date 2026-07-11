// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorkspaceStatsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_workspace_stats" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "workspace_id"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "dataset_count"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "tracer_session_count"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "repo_count"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "annotation_queue_count"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "deployment_count"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "dashboards_count"),
					resource.TestCheckResourceAttrSet("data.langsmith_workspace_stats.test", "evaluator_count"),
				),
			},
		},
	})
}
