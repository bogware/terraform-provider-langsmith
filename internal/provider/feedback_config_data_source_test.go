// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFeedbackConfigDataSource_basic creates a continuous feedback config
// and then looks it up by feedback key through the data source.
func TestAccFeedbackConfigDataSource_basic(t *testing.T) {
	rKey := fmt.Sprintf("tf-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFeedbackConfigDataSourceConfig(rKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_feedback_config.test", "feedback_key", rKey),
					resource.TestCheckResourceAttr("data.langsmith_feedback_config.test", "feedback_type", "continuous"),
					resource.TestCheckResourceAttr("data.langsmith_feedback_config.test", "min", "0"),
					resource.TestCheckResourceAttr("data.langsmith_feedback_config.test", "max", "1"),
					resource.TestCheckResourceAttrSet("data.langsmith_feedback_config.test", "modified_at"),
				),
			},
		},
	})
}

// testAccFeedbackConfigDataSourceConfig returns HCL that creates a feedback
// config and reads it back through the data source.
func testAccFeedbackConfigDataSourceConfig(key string) string {
	return fmt.Sprintf(`
resource "langsmith_feedback_config" "test" {
  feedback_key  = %[1]q
  feedback_type = "continuous"
  min           = 0
  max           = 1
}

data "langsmith_feedback_config" "test" {
  feedback_key = langsmith_feedback_config.test.feedback_key
}
`, key)
}
