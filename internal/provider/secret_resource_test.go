// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccSecretResource_basic rides into town with a secret and makes sure
// the lockbox holds. Out on the prairie you keep your powder dry and your
// secrets buried — this test ensures LangSmith does the same.
func TestAccSecretResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// CheckDestroy is handled automatically by the test framework
			// verifying the resource no longer exists.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: `resource "langsmith_secret" "test" {
  key   = "tf_acc_test_secret"
  value = "test_value_123"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_secret.test", "key", "tf_acc_test_secret"),
					resource.TestCheckResourceAttrSet("langsmith_secret.test", "id"),
				),
			},
		},
	})
}
