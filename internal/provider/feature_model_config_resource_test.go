// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFeatureModelConfigResource_basic mutates platform-wide feature model
// configuration, so it is opt-in. Set LANGSMITH_TEST_FEATURE to a valid
// platform feature name (see GET /v1/platform/features) to enable.
func TestAccFeatureModelConfigResource_basic(t *testing.T) {
	feature := os.Getenv("LANGSMITH_TEST_FEATURE")
	if feature == "" {
		t.Skip("Set LANGSMITH_TEST_FEATURE to a platform feature name to enable (mutates workspace-wide feature model config)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_feature_model_config" "test" {
  feature       = %[1]q
  default_model = "gpt-4o-mini"

  disabled_models = [
    "gpt-3.5-turbo",
  ]
}
`, feature),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_feature_model_config.test", "id", feature),
					resource.TestCheckResourceAttr("langsmith_feature_model_config.test", "feature", feature),
					resource.TestCheckResourceAttr("langsmith_feature_model_config.test", "default_model", "gpt-4o-mini"),
					resource.TestCheckResourceAttr("langsmith_feature_model_config.test", "disabled_models.#", "1"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "langsmith_feature_model_config" "test" {
  feature = %[1]q

  disabled_models = [
    "gpt-3.5-turbo",
    "gpt-4",
  ]
}
`, feature),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("langsmith_feature_model_config.test", "default_model"),
					resource.TestCheckResourceAttr("langsmith_feature_model_config.test", "disabled_models.#", "2"),
				),
			},
		},
	})
}
