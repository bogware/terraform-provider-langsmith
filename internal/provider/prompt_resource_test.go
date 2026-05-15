// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPromptResource_basic(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-prompt-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptResourceConfig(handle, false, "initial description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "id"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "repo_handle", handle),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "is_public", "false"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "description", "initial description"),
					// owner may be empty for prompts created via a service account.
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "full_name"),
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "tenant_id"),
					// counters have been removed in 0.9.0 — verify they're truly gone.
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_likes"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_views"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_downloads"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_commits"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "last_commit_hash"),
				),
			},
			{
				Config: testAccPromptResourceConfig(handle, false, "updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_prompt.test", "description", "updated description"),
				),
			},
			// Idempotency: the owner/full_name path fixes must produce zero diff on replay.
			{
				Config:             testAccPromptResourceConfig(handle, false, "updated description"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func testAccPromptResourceConfig(handle string, isPublic bool, description string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = %[2]t
  description = %[3]q
}
`, handle, isPublic, description)
}
