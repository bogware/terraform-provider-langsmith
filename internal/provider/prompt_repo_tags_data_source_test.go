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

func TestAccPromptRepoTagsDataSource_basic(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-prompt-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))
	tagName := "production"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptRepoTagsDataSourceConfig(handle, tagName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_prompt_repo_tags.test", "tags.#"),
					// The tag we created via langsmith_prompt_tag must show up in the list.
					resource.TestCheckTypeSetElemNestedAttrs(
						"data.langsmith_prompt_repo_tags.test",
						"tags.*",
						map[string]string{"tag_name": tagName},
					),
				),
			},
		},
	})
}

func testAccPromptRepoTagsDataSourceConfig(handle, tagName string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "prompt repo tags data source test"
  tags        = ["ChatPromptTemplate"]

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

resource "langsmith_prompt_tag" "test" {
  repo_handle = langsmith_prompt.test.repo_handle
  tag_name    = %[2]q
  commit_hash = langsmith_prompt.test.commit_hash
}

data "langsmith_prompt_repo_tags" "test" {
  repo_handle = langsmith_prompt.test.repo_handle
  depends_on  = [langsmith_prompt_tag.test]
}
`, handle, tagName)
}
