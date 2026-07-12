// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

func TestEffectiveClient(t *testing.T) {
	base := client.NewClient("https://api.smith.langchain.com", "key", "provider-ws", "ua", false)

	tests := []struct {
		name        string
		workspaceID types.String
		wantWS      string
		wantSame    bool
	}{
		{
			name:        "null returns original client",
			workspaceID: types.StringNull(),
			wantWS:      "provider-ws",
			wantSame:    true,
		},
		{
			name:        "unknown returns original client",
			workspaceID: types.StringUnknown(),
			wantWS:      "provider-ws",
			wantSame:    true,
		},
		{
			name:        "empty string returns original client",
			workspaceID: types.StringValue(""),
			wantWS:      "provider-ws",
			wantSame:    true,
		},
		{
			name:        "non-empty overrides workspace id",
			workspaceID: types.StringValue("override-ws"),
			wantWS:      "override-ws",
			wantSame:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveClient(base, tt.workspaceID)
			if got.WorkspaceID != tt.wantWS {
				t.Errorf("WorkspaceID = %q, want %q", got.WorkspaceID, tt.wantWS)
			}
			if (got == base) != tt.wantSame {
				t.Errorf("same-instance = %v, want %v", got == base, tt.wantSame)
			}
			// The original client must never be mutated.
			if base.WorkspaceID != "provider-ws" {
				t.Errorf("base client mutated: WorkspaceID = %q", base.WorkspaceID)
			}
		})
	}
}

func TestReconcileWorkspaceID(t *testing.T) {
	tests := []struct {
		name        string
		state       types.String
		apiValue    string
		wantState   types.String
		wantWarning bool
	}{
		{
			name:      "empty api value normalizes null to null",
			state:     types.StringNull(),
			apiValue:  "",
			wantState: types.StringNull(),
		},
		{
			name:      "empty api value normalizes unknown to null",
			state:     types.StringUnknown(),
			apiValue:  "",
			wantState: types.StringNull(),
		},
		{
			name:      "empty api value leaves a set value unchanged",
			state:     types.StringValue("user-ws"),
			apiValue:  "",
			wantState: types.StringValue("user-ws"),
		},
		{
			name:      "api value populates null state",
			state:     types.StringNull(),
			apiValue:  "api-ws",
			wantState: types.StringValue("api-ws"),
		},
		{
			name:      "api value populates unknown state",
			state:     types.StringUnknown(),
			apiValue:  "api-ws",
			wantState: types.StringValue("api-ws"),
		},
		{
			name:      "matching values are left unchanged without warning",
			state:     types.StringValue("api-ws"),
			apiValue:  "api-ws",
			wantState: types.StringValue("api-ws"),
		},
		{
			name:        "mismatch keeps user value and warns",
			state:       types.StringValue("user-ws"),
			apiValue:    "api-ws",
			wantState:   types.StringValue("user-ws"),
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			state := tt.state
			reconcileWorkspaceID(&state, tt.apiValue, &diags)

			if !state.Equal(tt.wantState) {
				t.Errorf("state = %#v, want %#v", state, tt.wantState)
			}
			if diags.WarningsCount() > 0 != tt.wantWarning {
				t.Errorf("warning present = %v, want %v (diags: %v)", diags.WarningsCount() > 0, tt.wantWarning, diags)
			}
			if diags.ErrorsCount() != 0 {
				t.Errorf("unexpected errors: %v", diags)
			}
		})
	}
}
