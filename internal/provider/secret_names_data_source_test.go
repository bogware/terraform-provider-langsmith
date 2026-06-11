// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccSecretNamesDataSource_basic creates a secret and verifies its key
// name shows up in the listing. Values are never returned by the API, so only
// names are checked.
func TestAccSecretNamesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_secret" "test" {
  key   = "TF_ACC_SECRET_NAMES_TEST"
  value = "test-value"
}

data "langsmith_secret_names" "test" {
  depends_on = [langsmith_secret.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_secret_names.test", "names.#"),
					resource.TestCheckTypeSetElemAttr("data.langsmith_secret_names.test", "names.*", "TF_ACC_SECRET_NAMES_TEST"),
				),
			},
		},
	})
}
