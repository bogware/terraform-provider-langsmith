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

// TestAccPersonalAccessTokenResource_basic requires a user-scoped credential —
// service keys cannot create PATs (the API requires a User ID). Set
// LANGSMITH_TEST_USER_KEY=1 when running with a user PAT/session credential.
func TestAccPersonalAccessTokenResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_USER_KEY") == "" {
		t.Skip("Set LANGSMITH_TEST_USER_KEY=1 to enable (PAT creation requires a user-scoped credential, not a service key)")
	}
	description := fmt.Sprintf("tf-pat-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_personal_access_token" "test" {
  description = %q
}
`, description),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_personal_access_token.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_personal_access_token.test", "key"),
					resource.TestCheckResourceAttrSet("langsmith_personal_access_token.test", "short_key"),
					resource.TestCheckResourceAttr("langsmith_personal_access_token.test", "description", description),
				),
			},
		},
	})
}
