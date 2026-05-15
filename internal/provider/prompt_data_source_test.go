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

func TestAccPromptDataSource_basic(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-prompt-ds-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "ds acceptance test"
}

data "langsmith_prompt" "test" {
  repo_handle = langsmith_prompt.test.repo_handle
}
`, handle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_prompt.test", "repo_handle", handle),
					resource.TestCheckResourceAttr("data.langsmith_prompt.test", "is_public", "false"),
					resource.TestCheckResourceAttr("data.langsmith_prompt.test", "description", "ds acceptance test"),
					resource.TestCheckResourceAttrSet("data.langsmith_prompt.test", "id"),
					resource.TestCheckResourceAttrSet("data.langsmith_prompt.test", "tenant_id"),
					// Counters were removed in 0.9.0 — confirm gone.
					resource.TestCheckNoResourceAttr("data.langsmith_prompt.test", "num_likes"),
					resource.TestCheckNoResourceAttr("data.langsmith_prompt.test", "num_commits"),
					resource.TestCheckNoResourceAttr("data.langsmith_prompt.test", "last_commit_hash"),
				),
			},
		},
	})
}
