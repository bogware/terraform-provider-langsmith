// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccComparativeExperimentResource_basic exercises create + delete of a
// comparative experiment. It is gated behind LANGSMITH_TEST_COMPARATIVE because
// it requires pre-existing experiments (sessions) and a reference dataset, which
// cannot be created within a Terraform acceptance test. Set:
//
//	LANGSMITH_TEST_COMPARATIVE=1
//	LANGSMITH_TEST_COMPARATIVE_DATASET_ID  — UUID of the reference dataset
//	LANGSMITH_TEST_COMPARATIVE_EXPERIMENT_1 — UUID of the first experiment
//	LANGSMITH_TEST_COMPARATIVE_EXPERIMENT_2 — UUID of the second experiment
func TestAccComparativeExperimentResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_COMPARATIVE") == "" {
		t.Skip("Set LANGSMITH_TEST_COMPARATIVE=1 (plus dataset/experiment IDs) to enable; requires pre-existing experiments")
	}
	datasetID := os.Getenv("LANGSMITH_TEST_COMPARATIVE_DATASET_ID")
	exp1 := os.Getenv("LANGSMITH_TEST_COMPARATIVE_EXPERIMENT_1")
	exp2 := os.Getenv("LANGSMITH_TEST_COMPARATIVE_EXPERIMENT_2")
	if datasetID == "" || exp1 == "" || exp2 == "" {
		t.Skip("Set LANGSMITH_TEST_COMPARATIVE_DATASET_ID, LANGSMITH_TEST_COMPARATIVE_EXPERIMENT_1 and LANGSMITH_TEST_COMPARATIVE_EXPERIMENT_2 to enable")
	}

	name := "tf-comparative-acc"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// No standalone GET-by-id endpoint exists for comparative
			// experiments (Read lists them per reference dataset), so destroy
			// verification is left to the framework's post-apply state diff.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccComparativeExperimentConfig(datasetID, exp1, exp2, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_comparative_experiment.test", "id"),
					resource.TestCheckResourceAttr("langsmith_comparative_experiment.test", "reference_dataset_id", datasetID),
					resource.TestCheckResourceAttr("langsmith_comparative_experiment.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_comparative_experiment.test", "experiment_ids.#", "2"),
					resource.TestCheckResourceAttrSet("langsmith_comparative_experiment.test", "created_at"),
				),
			},
			{
				ResourceName:            "langsmith_comparative_experiment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"experiment_ids"},
			},
		},
	})
}

func testAccComparativeExperimentConfig(datasetID, exp1, exp2, name string) string {
	return fmt.Sprintf(`
resource "langsmith_comparative_experiment" "test" {
  reference_dataset_id = %[1]q
  experiment_ids       = [%[2]q, %[3]q]
  name                 = %[4]q
  description          = "Created by an acceptance test."
}
`, datasetID, exp1, exp2, name)
}
