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

// TestAccAnnotationQueueReviewerResource_basic requires a workspace identity
// UUID to add as a reviewer. Set LANGSMITH_TEST_IDENTITY_ID to a real identity
// in the test workspace to enable.
func TestAccAnnotationQueueReviewerResource_basic(t *testing.T) {
	identityID := os.Getenv("LANGSMITH_TEST_IDENTITY_ID")
	if identityID == "" {
		t.Skip("Set LANGSMITH_TEST_IDENTITY_ID to enable this acceptance test")
	}

	queueName := fmt.Sprintf("tf-rev-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_annotation_queue" "test" {
  name = %[1]q
}

resource "langsmith_annotation_queue_reviewer" "test" {
  queue_id    = langsmith_annotation_queue.test.id
  identity_id = %[2]q
}
`, queueName, identityID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_annotation_queue_reviewer.test", "identity_id", identityID),
					resource.TestCheckResourceAttrSet("langsmith_annotation_queue_reviewer.test", "id"),
				),
			},
		},
	})
}
