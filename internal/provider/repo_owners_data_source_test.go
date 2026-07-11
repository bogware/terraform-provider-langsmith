// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRepoOwnersDataSource_basic creates a prompt repo and then reads its
// owners back through the data source, asserting that the read succeeds and the
// owners list is exposed. A repo created via a service-account API key has no
// human owners, so the list may legitimately be empty.
//
// Creating a prompt repo requires hub access and can be flaky on workspaces
// without it, so this test is gated behind LANGSMITH_TEST_HUB.
func TestAccRepoOwnersDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_HUB") == "" {
		t.Skip("Set LANGSMITH_TEST_HUB to enable (requires prompt hub access)")
	}

	handle := strings.ToLower(fmt.Sprintf("tf-owners-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRepoOwnersDataSourceConfig(handle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_repo_owners.test", "id"),
					resource.TestCheckResourceAttr("data.langsmith_repo_owners.test", "repo_handle", handle),
					resource.TestCheckResourceAttrSet("data.langsmith_repo_owners.test", "owner"),
					// The owners list is exposed (count attribute present). Owner
					// contents are not asserted: a service-account-created repo has
					// no human owners, so the list is legitimately empty here.
					resource.TestCheckResourceAttrSet("data.langsmith_repo_owners.test", "owners.#"),
				),
			},
		},
	})
}

func testAccRepoOwnersDataSourceConfig(handle string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "owners data source acc test"
}

data "langsmith_repo_owners" "test" {
  repo_handle = langsmith_prompt.test.repo_handle
}
`, handle)
}
