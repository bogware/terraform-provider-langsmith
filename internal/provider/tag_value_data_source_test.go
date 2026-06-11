// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagValueDataSource_basic(t *testing.T) {
	key := fmt.Sprintf("tf-tagkey-ds-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	value := fmt.Sprintf("tf-tagvalue-ds-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_tag_key" "test" {
  key = %[1]q
}

resource "langsmith_tag_value" "test" {
  tag_key_id  = langsmith_tag_key.test.id
  value       = %[2]q
  description = "tag value data source test"
}

data "langsmith_tag_value" "by_id" {
  tag_key_id = langsmith_tag_key.test.id
  id         = langsmith_tag_value.test.id
}

data "langsmith_tag_value" "by_value" {
  tag_key_id = langsmith_tag_key.test.id
  value      = langsmith_tag_value.test.value
}
`, key, value),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_tag_value.by_id", "value", value),
					resource.TestCheckResourceAttr("data.langsmith_tag_value.by_id", "description", "tag value data source test"),
					resource.TestCheckResourceAttrSet("data.langsmith_tag_value.by_id", "created_at"),
					resource.TestCheckResourceAttr("data.langsmith_tag_value.by_value", "value", value),
					resource.TestCheckResourceAttrPair(
						"data.langsmith_tag_value.by_value", "id",
						"langsmith_tag_value.test", "id",
					),
				),
			},
		},
	})
}
