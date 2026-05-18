// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFeedbackIngestTokenResource_basic requires an existing run UUID
// (LANGSMITH_TEST_RUN_ID) because the API does not let us create a run
// declaratively. Set it to a real, non-deleted run.
func TestAccFeedbackIngestTokenResource_basic(t *testing.T) {
	runID := os.Getenv("LANGSMITH_TEST_RUN_ID")
	if runID == "" {
		t.Skip("Set LANGSMITH_TEST_RUN_ID to a real run UUID to enable this acceptance test")
	}
	key := fmt.Sprintf("tf-fb-%s", acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_feedback_ingest_token" "test" {
  run_id       = %[1]q
  feedback_key = %[2]q
}
`, runID, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_feedback_ingest_token.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_feedback_ingest_token.test", "url"),
					resource.TestCheckResourceAttr("langsmith_feedback_ingest_token.test", "feedback_key", key),
				),
			},
		},
	})
}
