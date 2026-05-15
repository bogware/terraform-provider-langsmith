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

// TestAccRepoOwnerResource_basic adds another existing-tenant user as a repo
// owner. Set LANGSMITH_TEST_OWNER_EMAIL + LANGSMITH_TEST_TENANT_HANDLE to a
// second user's email + the workspace's tenant handle to enable.
func TestAccRepoOwnerResource_basic(t *testing.T) {
	email := os.Getenv("LANGSMITH_TEST_OWNER_EMAIL")
	owner := os.Getenv("LANGSMITH_TEST_TENANT_HANDLE")
	if email == "" || owner == "" {
		t.Skip("Set LANGSMITH_TEST_OWNER_EMAIL and LANGSMITH_TEST_TENANT_HANDLE to enable this acceptance test")
	}

	repoName := strings.ToLower(fmt.Sprintf("tf-repo-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  description = "tf-test prompt"
}

resource "langsmith_repo_owner" "test" {
  owner = %[2]q
  repo  = langsmith_prompt.test.repo_handle
  email = %[3]q
}
`, repoName, owner, email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_repo_owner.test", "email", email),
					resource.TestCheckResourceAttrSet("langsmith_repo_owner.test", "ls_user_id"),
				),
			},
		},
	})
}
