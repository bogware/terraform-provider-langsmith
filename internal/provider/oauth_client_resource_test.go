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

// TestAccOAuthClientResource_basic is opt-in: the OAuth client endpoints return
// 404 unless the organization is entitled to them, so this cannot run on a
// standard workspace.
func TestAccOAuthClientResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_OAUTH") == "" {
		t.Skip("Set LANGSMITH_TEST_OAUTH=1 to enable (requires an org entitled to OAuth client registration)")
	}
	name := fmt.Sprintf("tf-acc-oauth-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOAuthClientConfig(name, "https://example.com/callback"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_oauth_client.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_oauth_client.test", "client_id"),
					resource.TestCheckResourceAttrSet("langsmith_oauth_client.test", "client_secret"),
					resource.TestCheckResourceAttr("langsmith_oauth_client.test", "client_name", name),
					resource.TestCheckResourceAttr("langsmith_oauth_client.test", "client_type", "confidential"),
				),
			},
			// redirect_uris is accepted by the update endpoint, so this must apply
			// in place and leave the issued secret untouched.
			{
				Config: testAccOAuthClientConfig(name, "https://example.com/other-callback"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_oauth_client.test", "redirect_uris.0", "https://example.com/other-callback"),
					resource.TestCheckResourceAttrSet("langsmith_oauth_client.test", "client_secret"),
				),
			},
		},
	})
}

func testAccOAuthClientConfig(name, redirect string) string {
	return fmt.Sprintf(`
resource "langsmith_oauth_client" "test" {
  client_name   = %[1]q
  client_type   = "confidential"
  redirect_uris = [%[2]q]
  grant_types   = ["authorization_code"]
}
`, name, redirect)
}
