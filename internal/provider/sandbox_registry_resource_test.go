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

// TestAccSandboxRegistryResource_basic creates a real sandbox registry
// credential, updates its URL, and imports it back.
//
// This one is gated: it needs a real registry URL and credentials, so it only
// saddles up when LANGSMITH_TEST_SANDBOX_REGISTRY is set. The URL/username/
// password come from the environment when supplied, with harmless placeholders
// otherwise (the API stores the credentials; it does not verify them at create
// time).
func TestAccSandboxRegistryResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_SANDBOX_REGISTRY") == "" {
		t.Skip("LANGSMITH_TEST_SANDBOX_REGISTRY not set; skipping sandbox registry resource test (requires a real registry URL and credentials)")
	}

	name := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	registryURL := sandboxRegistryEnv("LANGSMITH_TEST_SANDBOX_REGISTRY_URL", "registry.example.com")
	updatedURL := sandboxRegistryEnv("LANGSMITH_TEST_SANDBOX_REGISTRY_URL_UPDATED", "registry-2.example.com")
	username := sandboxRegistryEnv("LANGSMITH_TEST_SANDBOX_REGISTRY_USERNAME", "tf-acc-user")
	password := sandboxRegistryEnv("LANGSMITH_TEST_SANDBOX_REGISTRY_PASSWORD", "tf-acc-password")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSandboxRegistryResourceConfig(name, registryURL, username, password),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_sandbox_registry.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_sandbox_registry.test", "url", registryURL),
					resource.TestCheckResourceAttr("langsmith_sandbox_registry.test", "username", username),
					resource.TestCheckResourceAttr("langsmith_sandbox_registry.test", "password", password),
					resource.TestCheckResourceAttrSet("langsmith_sandbox_registry.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_sandbox_registry.test", "workspace_id"),
				),
			},
			// Update the URL in place -- name is RequiresReplace, url is not.
			{
				Config: testAccSandboxRegistryResourceConfig(name, updatedURL, username, password),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_sandbox_registry.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_sandbox_registry.test", "url", updatedURL),
				),
			},
			// Import by name. username/password are write-only and are never
			// returned by the API, so they cannot round-trip through an import.
			{
				ResourceName:            "langsmith_sandbox_registry.test",
				ImportState:             true,
				ImportStateId:           name,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"username", "password"},
			},
		},
	})
}

func testAccSandboxRegistryResourceConfig(name, registryURL, username, password string) string {
	return fmt.Sprintf(`
resource "langsmith_sandbox_registry" "test" {
  name     = %[1]q
  url      = %[2]q
  username = %[3]q
  password = %[4]q
}
`, name, registryURL, username, password)
}

// sandboxRegistryEnv returns the value of the environment variable, or fallback when
// it is unset or empty.
func sandboxRegistryEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
