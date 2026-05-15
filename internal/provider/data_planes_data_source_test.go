// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccDataPlanesDataSource_basic targets /orgs/current/data-planes. The
// endpoint is in the OpenAPI but returns 404 on the public hosted API
// (BYOC-only). Set LANGSMITH_TEST_DATA_PLANES_ENABLED=1 when targeting an
// instance that exposes it.
func TestAccDataPlanesDataSource_basic(t *testing.T) {
	if os.Getenv("LANGSMITH_TEST_DATA_PLANES_ENABLED") == "" {
		t.Skip("Set LANGSMITH_TEST_DATA_PLANES_ENABLED=1 to enable (BYOC-only endpoint; 404s on standard tier)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_data_planes" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.langsmith_data_planes.test", "data_planes.#"),
				),
			},
		},
	})
}
