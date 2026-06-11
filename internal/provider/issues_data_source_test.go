// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccIssuesDataSource_basic targets the beta /v1/platform/issues endpoint.
// It is not present in all deployments (it is absent from the published
// OpenAPI spec), so it is opt-in via LANGSMITH_TEST_ISSUES_ENABLED=1.
func TestAccIssuesDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_ISSUES_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_ISSUES_ENABLED=1 to enable (beta endpoint; may 404 on some deployments)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_issues" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_issues.test", "issues.#"),
				),
			},
		},
	})
}
