// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSettingsResourceApplyDesiredHandle_RejectsWhitespaceOnly(t *testing.T) {
	t.Parallel()

	r := &SettingsResource{}
	data := &SettingsResourceModel{
		TenantHandle: types.StringValue("   "),
	}

	diags := r.applyDesiredHandle(context.Background(), data)
	if !diags.HasError() {
		t.Fatal("expected diagnostics error for whitespace-only tenant_handle")
	}
	if got := diags[0].Detail(); !strings.Contains(got, "non-whitespace character") {
		t.Fatalf("unexpected diagnostic detail: %q", got)
	}
}

func TestSettingsResourceApplyDesiredHandle_RejectsLeadingTrailingWhitespace(t *testing.T) {
	t.Parallel()

	r := &SettingsResource{}
	data := &SettingsResourceModel{
		TenantHandle: types.StringValue("  valid-handle  "),
	}

	diags := r.applyDesiredHandle(context.Background(), data)
	if !diags.HasError() {
		t.Fatal("expected diagnostics error for tenant_handle with surrounding whitespace")
	}
	if got := diags[0].Detail(); !strings.Contains(got, "leading or trailing whitespace") {
		t.Fatalf("unexpected diagnostic detail: %q", got)
	}
}
