// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestReadStringSet(t *testing.T) {
	ctx := context.Background()

	t.Run("null", func(t *testing.T) {
		s := types.SetNull(types.StringType)
		out, err := readStringSet(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			t.Fatalf("got %v, want nil", out)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		s := types.SetUnknown(types.StringType)
		out, err := readStringSet(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			t.Fatalf("got %v, want nil", out)
		}
	})

	t.Run("values", func(t *testing.T) {
		set, diags := types.SetValueFrom(ctx, types.StringType, []string{"a", "b"})
		if diags.HasError() {
			t.Fatal(diags)
		}
		out, err := readStringSet(ctx, set)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 2 {
			t.Fatalf("len = %d", len(out))
		}
		have := map[string]bool{}
		for _, s := range out {
			have[s] = true
		}
		if !have["a"] || !have["b"] {
			t.Fatalf("got %#v", out)
		}
	})
}
