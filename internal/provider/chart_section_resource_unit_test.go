// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRestoreChartSectionUpdatedAtFromPrior(t *testing.T) {
	t.Run("keeps value from api response", func(t *testing.T) {
		data := ChartSectionResourceModel{UpdatedAt: types.StringValue("2026-01-02T00:00:00Z")}
		restoreChartSectionUpdatedAtFromPrior(&data, types.StringValue("2026-01-01T00:00:00Z"))
		if got := data.UpdatedAt.ValueString(); got != "2026-01-02T00:00:00Z" {
			t.Fatalf("expected updated_at from API response, got %q", got)
		}
	})

	t.Run("falls back to prior when api omits updated_at", func(t *testing.T) {
		data := ChartSectionResourceModel{UpdatedAt: types.StringNull()}
		restoreChartSectionUpdatedAtFromPrior(&data, types.StringValue("2026-01-01T00:00:00Z"))
		if got := data.UpdatedAt.ValueString(); got != "2026-01-01T00:00:00Z" {
			t.Fatalf("expected fallback updated_at, got %q", got)
		}
	})
}
