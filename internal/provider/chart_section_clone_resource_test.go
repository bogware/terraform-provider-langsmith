// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccChartSectionCloneResource_basic(t *testing.T) {
	sourceTitle := fmt.Sprintf("tf-source-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	cloneTitle := fmt.Sprintf("tf-clone-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccChartSectionCloneResourceConfig(sourceTitle, cloneTitle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_chart_section_clone.test", "id"),
					resource.TestCheckResourceAttr("langsmith_chart_section_clone.test", "title", cloneTitle),
					resource.TestCheckResourceAttrPair(
						"langsmith_chart_section_clone.test", "source_section_id",
						"langsmith_chart_section.source", "id",
					),
				),
			},
			{
				ResourceName:      "langsmith_chart_section_clone.test",
				ImportState:       true,
				ImportStateVerify: true,
				// source_section_id and session_id are clone-time inputs not returned by the API.
				// created_at/updated_at are returned by the clone endpoint but not by the
				// single-section read endpoint, so they go null on import.
				ImportStateVerifyIgnore: []string{"source_section_id", "session_id", "created_at", "updated_at"},
			},
		},
	})
}

func testAccChartSectionCloneResourceConfig(sourceTitle, cloneTitle string) string {
	return fmt.Sprintf(`
resource "langsmith_chart_section" "source" {
  title       = %[1]q
  description = "original"
}

resource "langsmith_chart_section_clone" "test" {
  source_section_id = langsmith_chart_section.source.id
  title             = %[2]q
}
`, sourceTitle, cloneTitle)
}
