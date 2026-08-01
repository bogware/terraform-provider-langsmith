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

// TestAccHubDirectoryResource_basic writes real commits to a hub repo, so it is
// opt-in: set LANGSMITH_TEST_HUB_DIRECTORY=1 and LANGSMITH_TEST_HUB_DIRECTORY_REPO
// to a repo that may be committed to and deleted.
func TestAccHubDirectoryResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_HUB_DIRECTORY") == "" {
		t.Skip("Set LANGSMITH_TEST_HUB_DIRECTORY=1 to enable (writes commits to a hub repo and deletes it on destroy)")
	}
	repo := os.Getenv("LANGSMITH_TEST_HUB_DIRECTORY_REPO")
	if repo == "" {
		repo = fmt.Sprintf("tf-acc-dir-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha))
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHubDirectoryConfig(repo, "# first\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_hub_directory.test", "repo", repo),
					resource.TestCheckResourceAttrSet("langsmith_hub_directory.test", "commit_hash"),
				),
			},
			// A second commit must build on the first rather than replace the
			// resource.
			{
				Config: testAccHubDirectoryConfig(repo, "# second\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_hub_directory.test", "commit_hash"),
				),
			},
		},
	})
}

func testAccHubDirectoryConfig(repo, readme string) string {
	return fmt.Sprintf(`
resource "langsmith_hub_directory" "test" {
  owner = "-"
  repo  = %[1]q
  files = jsonencode({ "README.md" = %[2]q })
}
`, repo, readme)
}
