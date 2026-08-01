// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func mapOverrides(t *testing.T, in map[string]string) types.Map {
	t.Helper()
	m, diags := types.MapValueFrom(context.Background(), types.StringType, in)
	if diags.HasError() {
		t.Fatalf("building map value: %v", diags)
	}
	return m
}

func TestGetPathOverrides(t *testing.T) {
	cases := []struct {
		name    string
		attr    *map[string]string // nil => attribute is null
		env     string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "unset everywhere yields nil",
			want: nil,
		},
		{
			name: "from attribute",
			attr: &map[string]string{"/api/v1/platform/": "/v1/platform/"},
			want: map[string]string{"/api/v1/platform/": "/v1/platform/"},
		},
		{
			name: "from env",
			env:  `{"/api/v1/platform/":"/v1/platform/"}`,
			want: map[string]string{"/api/v1/platform/": "/v1/platform/"},
		},
		{
			// A non-null attribute wins over the environment, matching every other
			// provider attribute.
			name: "attribute overrides env",
			attr: &map[string]string{"/workspaces/": "/api/workspaces/"},
			env:  `{"/api/v1/platform/":"/v1/platform/"}`,
			want: map[string]string{"/workspaces/": "/api/workspaces/"},
		},
		{
			name: "empty attribute map yields nil",
			attr: &map[string]string{},
			want: nil,
		},
		{
			name:    "malformed env json",
			env:     `not json`,
			wantErr: true,
		},
		{
			name:    "prefix without leading slash",
			attr:    &map[string]string{"api/v1/": "/api/v1/"},
			wantErr: true,
		},
		{
			name:    "replacement without leading slash",
			attr:    &map[string]string{"/api/v1/": "api/v1/"},
			wantErr: true,
		},
		{
			name:    "empty prefix matches everything",
			attr:    &map[string]string{"": "/api/"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LANGSMITH_PATH_OVERRIDES", tc.env)

			data := LangSmithProviderModel{PathOverrides: types.MapNull(types.StringType)}
			if tc.attr != nil {
				data.PathOverrides = mapOverrides(t, *tc.attr)
			}

			resp := &provider.ConfigureResponse{}
			got := getPathOverrides(context.Background(), data, resp)

			if tc.wantErr {
				if !resp.Diagnostics.HasError() {
					t.Fatalf("expected an error diagnostic, got none (result %v)", got)
				}
				return
			}
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("got[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
