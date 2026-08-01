// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccOptimizationJobResource_basic runs a real optimization job, which
// consumes model spend, so it is opt-in. Set LANGSMITH_TEST_OPTIMIZATION=1 plus
// LANGSMITH_TEST_OPTIMIZATION_REPO (an existing hub repo handle) and
// LANGSMITH_TEST_OPTIMIZATION_DATASET_ID.
func TestAccOptimizationJobResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_OPTIMIZATION") == "" {
		t.Skip("Set LANGSMITH_TEST_OPTIMIZATION=1 to enable (runs a real optimization job and incurs model spend)")
	}
	repo := os.Getenv("LANGSMITH_TEST_OPTIMIZATION_REPO")
	dataset := os.Getenv("LANGSMITH_TEST_OPTIMIZATION_DATASET_ID")
	if repo == "" || dataset == "" {
		t.Skip("Set LANGSMITH_TEST_OPTIMIZATION_REPO and LANGSMITH_TEST_OPTIMIZATION_DATASET_ID to enable")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_optimization_job" "test" {
  owner     = "-"
  repo      = %[1]q
  algorithm = "demo"
  config    = jsonencode({ dataset_id = %[2]q })
}

data "langsmith_optimization_job_logs" "test" {
  owner  = langsmith_optimization_job.test.owner
  repo   = langsmith_optimization_job.test.repo
  job_id = langsmith_optimization_job.test.id
}
`, repo, dataset),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_optimization_job.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_optimization_job.test", "status"),
					resource.TestCheckResourceAttr("langsmith_optimization_job.test", "algorithm", "demo"),
					resource.TestCheckResourceAttrSet("data.langsmith_optimization_job_logs.test", "logs.#"),
				),
			},
		},
	})
}
