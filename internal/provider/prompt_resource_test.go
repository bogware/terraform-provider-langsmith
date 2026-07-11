// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
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
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "workspace_id"),
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

// TestAccPromptResource_partialCreateRecovery is a regression test for issue
// #61: the repo POST succeeds but the follow-up commit POST fails (unsupported
// manifest type). The provider must persist partial state so the repo is
// tracked (tainted) and replaced on the next apply, instead of being orphaned
// remotely and causing a 409 "already exists" on the retry.
func TestAccPromptResource_partialCreateRecovery(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-prompt-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: RunnableSequence manifests are rejected by the commit
			// endpoint with 400 "Manifest type ... is not supported", after the
			// repo itself has already been created.
			{
				Config:      testAccPromptResourceConfigUnsupportedManifest(handle),
				ExpectError: regexp.MustCompile("is not supported"),
			},
			// Step 2: same resource address with a valid ChatPromptTemplate
			// manifest. The tainted repo from step 1 is destroyed and
			// recreated; this must apply cleanly with no 409 conflict.
			{
				Config: testAccPromptResourceConfigValidManifest(handle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "id"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "repo_handle", handle),
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "commit_hash"),
				),
			},
		},
	})
}

func testAccPromptResourceConfigUnsupportedManifest(handle string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "partial create recovery test"

  manifest = jsonencode({
    lc     = 1
    type   = "constructor"
    id     = ["langchain", "schema", "runnable", "RunnableSequence"]
    kwargs = {}
  })
}
`, handle)
}

func testAccPromptResourceConfigValidManifest(handle string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "partial create recovery test"
  # The server auto-tags the repo with the manifest type on commit; declare
  # it so the post-apply refresh plan is empty.
  tags = ["ChatPromptTemplate"]

  manifest = jsonencode({
    lc   = 1
    type = "constructor"
    id   = ["langchain", "prompts", "chat", "ChatPromptTemplate"]
    kwargs = {
      input_variables = ["question"]
      messages = [
        {
          lc   = 1
          type = "constructor"
          id   = ["langchain", "prompts", "chat", "HumanMessagePromptTemplate"]
          kwargs = {
            prompt = {
              lc   = 1
              type = "constructor"
              id   = ["langchain", "prompts", "prompt", "PromptTemplate"]
              kwargs = {
                input_variables = ["question"]
                template        = "{question}"
                template_format = "f-string"
              }
            }
          }
        }
      ]
    }
  })
}
`, handle)
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
