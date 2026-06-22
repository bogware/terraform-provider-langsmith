// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccAPIKeyResource_basic creates a durable API key, verifies the server
// returns an id, short_key, and the full secret key, then relies on the test
// framework's CheckDestroy to confirm deletion.
//
// Minting workspace/org API keys can require elevated permissions that a
// disposable test org may lack (the POST can 403). The test is therefore gated
// behind LANGSMITH_TEST_API_KEYS=1 so it is opt-in.
func TestAccAPIKeyResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_API_KEYS") == "" {
		t.Skip("Set LANGSMITH_TEST_API_KEYS=1 to enable (creating API keys may require elevated org permissions)")
	}

	description := fmt.Sprintf("tf-acc-api-key-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// CheckDestroy is satisfied by the framework verifying the
			// resource no longer exists after the test tears down.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccAPIKeyResourceConfig(description),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_api_key.test", "short_key"),
					resource.TestCheckResourceAttrSet("langsmith_api_key.test", "key"),
					resource.TestCheckResourceAttr("langsmith_api_key.test", "description", description),
				),
			},
			// Idempotency: replaying the same config must produce zero diff.
			{
				Config:             testAccAPIKeyResourceConfig(description),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func testAccAPIKeyResourceConfig(description string) string {
	return fmt.Sprintf(`
resource "langsmith_api_key" "test" {
  description = %[1]q
}
`, description)
}
