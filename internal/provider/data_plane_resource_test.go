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

// TestAccDataPlaneResource_basic provisions a real BYOC data plane, which the
// API cannot delete — the created data plane LEAKS and must be deprovisioned
// via LangSmith support. It is therefore strictly opt-in: set
// LANGSMITH_TEST_DATA_PLANE_ENABLED=1, LANGSMITH_TEST_DATA_PLANE_ROLE_ARN, and
// LANGSMITH_TEST_DATA_PLANE_EXTERNAL_ID on a BYOC-enabled org to enable.
func TestAccDataPlaneResource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_DATA_PLANE_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_DATA_PLANE_ENABLED=1 to enable (requires a BYOC org; the created data plane cannot be deleted via the API)")
	}
	roleARN := os.Getenv("LANGSMITH_TEST_DATA_PLANE_ROLE_ARN")
	externalID := os.Getenv("LANGSMITH_TEST_DATA_PLANE_EXTERNAL_ID")
	if roleARN == "" || externalID == "" {
		t.Skip("Set LANGSMITH_TEST_DATA_PLANE_ROLE_ARN and LANGSMITH_TEST_DATA_PLANE_EXTERNAL_ID to enable")
	}
	name := fmt.Sprintf("tf-dp-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_data_plane" "test" {
  name        = %[1]q
  region      = "us-east-1"
  external_id = %[2]q
  role_arn    = %[3]q
  vpc_cidr    = "10.42.0.0/16"
}
`, name, externalID, roleARN),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_data_plane.test", "id"),
					resource.TestCheckResourceAttr("langsmith_data_plane.test", "name", name),
					resource.TestCheckResourceAttr("langsmith_data_plane.test", "status", "requested"),
				),
			},
		},
	})
}
