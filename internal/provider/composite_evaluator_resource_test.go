// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// TestAccCompositeEvaluatorResource_basic covers the replacement for the
// retired langsmith_feedback_formula (#98): create on a project, replace when
// the formula changes (there is no update endpoint), import, and destroy.
// CheckDestroy confirms both LangSmith objects are gone, since deleting only
// the run rule would leave the evaluator behind.
func TestAccCompositeEvaluatorResource_basic(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	var firstEvaluatorID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCompositeEvaluatorDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccCompositeEvaluatorConfig(rName, "avg", 0.5),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_composite_evaluator.test", "rule_id"),
					resource.TestCheckResourceAttrSet("langsmith_composite_evaluator.test", "evaluator_id"),
					resource.TestCheckResourceAttrPair("langsmith_composite_evaluator.test", "id",
						"langsmith_composite_evaluator.test", "rule_id"),
					resource.TestCheckResourceAttrSet("langsmith_composite_evaluator.test", "workspace_id"),
					func(s *terraform.State) error {
						firstEvaluatorID = s.RootModule().Resources["langsmith_composite_evaluator.test"].Primary.Attributes["evaluator_id"]
						return nil
					},
				),
			},
			{
				// A changed weight cannot be updated in place, so the pair is
				// replaced and the old evaluator must be gone.
				Config: testAccCompositeEvaluatorConfig(rName, "sum", 2.0),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_composite_evaluator.test", "aggregation_type", "sum"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["langsmith_composite_evaluator.test"].Primary.Attributes["evaluator_id"]
						if id == firstEvaluatorID {
							return fmt.Errorf("evaluator_id unchanged (%s); expected replacement", id)
						}
						return testAccCompositeEvaluatorGone(firstEvaluatorID)
					},
				),
			},
			{
				ResourceName:      "langsmith_composite_evaluator.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					a := s.RootModule().Resources["langsmith_composite_evaluator.test"].Primary.Attributes
					return a["rule_id"] + "/" + a["evaluator_id"], nil
				},
				// LangSmith does not return the formula inputs.
				ImportStateVerifyIgnore: []string{"feedback_key", "aggregation_type", "formula_parts", "session_id", "dataset_id"},
			},
		},
	})
}

// TestAccCompositeEvaluatorResource_dataset covers dataset scoping.
func TestAccCompositeEvaluatorResource_dataset(t *testing.T) {
	rName := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCompositeEvaluatorDestroyed,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_dataset" "test" {
  name      = "tf-acc-composite-%[1]s"
  data_type = "kv"
}

resource "langsmith_composite_evaluator" "test" {
  feedback_key     = "tf_composite_%[1]s"
  aggregation_type = "avg"
  dataset_id       = langsmith_dataset.test.id
  formula_parts = jsonencode([
    { part_type = "weighted_key", weight = 1.0, key = "correctness" },
  ])
}
`, rName),
				Check: resource.TestCheckResourceAttrSet("langsmith_composite_evaluator.test", "evaluator_id"),
			},
		},
	})
}

// TestAccCompositeEvaluatorResource_scopeValidation checks that exactly one of
// session_id and dataset_id is required, at plan time.
func TestAccCompositeEvaluatorResource_scopeValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_composite_evaluator" "test" {
  feedback_key     = "composite"
  aggregation_type = "avg"
  formula_parts    = jsonencode([{ part_type = "weighted_key", weight = 1.0, key = "correctness" }])
}
`,
				ExpectError: regexp.MustCompile(`Missing Attribute Configuration`),
			},
		},
	})
}

func testAccCompositeEvaluatorConfig(rName, aggregation string, weight float64) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = "tf-acc-composite-%[1]s"
}

resource "langsmith_composite_evaluator" "test" {
  feedback_key     = "tf_composite_%[1]s"
  aggregation_type = %[2]q
  session_id       = langsmith_project.test.id
  formula_parts = jsonencode([
    { part_type = "weighted_key", weight = 1.0, key = "correctness" },
    { part_type = "weighted_key", weight = %[3]g, key = "helpfulness" },
  ])
}
`, rName, aggregation, weight)
}

func testAccCompositeEvaluatorClient() *client.Client {
	url := os.Getenv("LANGSMITH_API_URL")
	if url == "" {
		url = "https://api.smith.langchain.com"
	}
	return client.NewClient(url, os.Getenv("LANGSMITH_API_KEY"), os.Getenv("LANGSMITH_WORKSPACE_ID"), "tf-acc-test", false, nil)
}

// testAccCompositeEvaluatorGone returns an error unless the evaluator 404s.
func testAccCompositeEvaluatorGone(evaluatorID string) error {
	var out map[string]any
	err := testAccCompositeEvaluatorClient().Get(context.Background(), "/api/v1/platform/evaluators/"+evaluatorID, nil, &out)
	if err == nil {
		return fmt.Errorf("evaluator %s still exists", evaluatorID)
	}
	if !client.IsNotFound(err) {
		return fmt.Errorf("checking evaluator %s: %w", evaluatorID, err)
	}
	return nil
}

// testAccCheckCompositeEvaluatorDestroyed confirms both the evaluator and the
// run rule are gone for every composite evaluator in the final state.
func testAccCheckCompositeEvaluatorDestroyed(s *terraform.State) error {
	var rules []struct {
		ID string `json:"id"`
	}
	if err := testAccCompositeEvaluatorClient().Get(context.Background(), "/api/v1/runs/rules", nil, &rules); err != nil {
		return fmt.Errorf("listing run rules: %w", err)
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "langsmith_composite_evaluator" {
			continue
		}
		if err := testAccCompositeEvaluatorGone(rs.Primary.Attributes["evaluator_id"]); err != nil {
			return err
		}
		for _, rule := range rules {
			if rule.ID == rs.Primary.Attributes["rule_id"] {
				return fmt.Errorf("run rule %s still exists", rule.ID)
			}
		}
	}
	return nil
}
