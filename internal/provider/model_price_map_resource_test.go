// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestModelPriceMapResource_matchPathComputed guards #89. LangSmith fills in a
// default match_path when it is omitted, so the attribute must be Computed:
// as Optional alone, the first apply failed with "Provider produced
// inconsistent result after apply: .match_path: was null, but now ...".
func TestModelPriceMapResource_matchPathComputed(t *testing.T) {
	var resp resource.SchemaResponse
	NewModelPriceMapResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["match_path"].(schema.ListAttribute)
	if !ok {
		t.Fatalf("match_path is %T, want schema.ListAttribute", resp.Schema.Attributes["match_path"])
	}
	if !attr.Optional || !attr.Computed {
		t.Fatalf("match_path Optional=%v Computed=%v, want both true", attr.Optional, attr.Computed)
	}
	if len(attr.PlanModifiers) == 0 {
		t.Fatal("match_path has no plan modifiers, want UseStateForUnknown so an unchanged value does not plan as unknown")
	}
}
