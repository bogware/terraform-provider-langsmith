// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNormalizeTimestamp(t *testing.T) {
	cases := []struct {
		name     string
		apiValue string
		saved    types.String
		want     types.String
	}{
		{
			name:     "empty is null",
			apiValue: "",
			saved:    types.StringNull(),
			want:     types.StringNull(),
		},
		{
			// Import starts from empty state, so the zone-less create response and
			// the zoned read response have to canonicalise to the same string or
			// ImportStateVerify reports a phantom difference.
			name:     "zoneless canonicalises to UTC",
			apiValue: "2026-07-31T23:42:26.982665",
			saved:    types.StringNull(),
			want:     types.StringValue("2026-07-31T23:42:26.982665Z"),
		},
		{
			name:     "offset canonicalises to the same string",
			apiValue: "2026-07-31T23:42:26.982665+00:00",
			saved:    types.StringNull(),
			want:     types.StringValue("2026-07-31T23:42:26.982665Z"),
		},
		{
			name:     "non-UTC offset is converted",
			apiValue: "2026-07-31T18:42:26-05:00",
			saved:    types.StringNull(),
			want:     types.StringValue("2026-07-31T23:42:26Z"),
		},
		{
			// The practitioner's spelling survives when it means the same instant.
			name:     "same instant keeps the saved spelling",
			apiValue: "2026-07-31T23:42:26.982665+00:00",
			saved:    types.StringValue("2026-07-31T23:42:26.982665Z"),
			want:     types.StringValue("2026-07-31T23:42:26.982665Z"),
		},
		{
			name:     "different instant takes the API value",
			apiValue: "2026-07-31T23:42:26Z",
			saved:    types.StringValue("2020-01-01T00:00:00Z"),
			want:     types.StringValue("2026-07-31T23:42:26Z"),
		},
		{
			name:     "unparseable passes through",
			apiValue: "not a timestamp",
			saved:    types.StringNull(),
			want:     types.StringValue("not a timestamp"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTimestamp(tc.apiValue, tc.saved)
			if !got.Equal(tc.want) {
				t.Fatalf("normalizeTimestamp(%q, %v) = %v, want %v", tc.apiValue, tc.saved, got, tc.want)
			}
		})
	}
}

func TestJSONPreserveConfigSubset(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		saved types.String
		want  types.String
	}{
		{
			// The case this exists for: the server adds language to a code
			// evaluator the practitioner submitted without one.
			name:  "server adds a key",
			raw:   `[{"code":"x","language":"python"}]`,
			saved: types.StringValue(`[{"code":"x"}]`),
			want:  types.StringValue(`[{"code":"x"}]`),
		},
		{
			name:  "server changes a configured value",
			raw:   `[{"code":"y","language":"python"}]`,
			saved: types.StringValue(`[{"code":"x"}]`),
			want:  types.StringValue(`[{"code":"y","language":"python"}]`),
		},
		{
			name:  "element added server-side",
			raw:   `[{"code":"x"},{"code":"z"}]`,
			saved: types.StringValue(`[{"code":"x"}]`),
			want:  types.StringValue(`[{"code":"x"},{"code":"z"}]`),
		},
		{
			name:  "no saved value takes the response",
			raw:   `[{"code":"x","language":"python"}]`,
			saved: types.StringNull(),
			want:  types.StringValue(`[{"code":"x","language":"python"}]`),
		},
		{
			name:  "empty array is null",
			raw:   `[]`,
			saved: types.StringValue(`[{"code":"x"}]`),
			want:  types.StringNull(),
		},
		{
			name:  "nested object subset",
			raw:   `{"a":{"b":1,"c":2}}`,
			saved: types.StringValue(`{"a":{"b":1}}`),
			want:  types.StringValue(`{"a":{"b":1}}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonPreserveConfigSubset([]byte(tc.raw), tc.saved)
			if tc.want.IsNull() {
				if !got.IsNull() {
					t.Fatalf("got %v, want null", got)
				}
				return
			}
			if normalizeJSON(got.ValueString()) != normalizeJSON(tc.want.ValueString()) {
				t.Fatalf("got %s, want %s", got.ValueString(), tc.want.ValueString())
			}
		})
	}
}
