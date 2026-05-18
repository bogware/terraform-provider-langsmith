// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestInsightsConfigResource_basePath(t *testing.T) {
	t.Parallel()
	var r InsightsConfigResource
	got := r.basePath("sess-1")
	want := "/api/v1/sessions/sess-1/insights/configs"
	if got != want {
		t.Fatalf("basePath = %q, want %q", got, want)
	}
}
