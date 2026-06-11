// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccWorkspaceHandleResource_basic is gated behind an explicit opt-in
// because workspace handles are globally unique and cannot be unset once
// assigned: applying this test permanently sets (or changes) the handle of
// the workspace the credentials point at. Set LANGSMITH_TEST_SET_WORKSPACE_HANDLE
// to the handle value you want assigned to enable it.
func TestAccWorkspaceHandleResource_basic(t *testing.T) {
	handle := os.Getenv("LANGSMITH_TEST_SET_WORKSPACE_HANDLE")
	if handle == "" {
		t.Skip("Set LANGSMITH_TEST_SET_WORKSPACE_HANDLE to enable (permanently assigns the handle to the test workspace)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceHandleResourceConfig(handle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_workspace_handle.test", "handle", handle),
					resource.TestCheckResourceAttrSet("langsmith_workspace_handle.test", "id"),
					resource.TestCheckResourceAttrSet("langsmith_workspace_handle.test", "display_name"),
				),
			},
		},
	})
}

func testAccWorkspaceHandleResourceConfig(handle string) string {
	return fmt.Sprintf(`
resource "langsmith_workspace_handle" "test" {
  handle = %[1]q
}
`, handle)
}
