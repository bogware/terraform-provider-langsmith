// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// effectiveClient returns a shallow copy of c with WorkspaceID set to the
// resource-level override when the caller has supplied a non-empty workspace_id.
// If workspaceID is null, unknown, or the empty string the original client is
// returned unchanged, so the provider-level WorkspaceID (or none) is used.
func effectiveClient(c *client.Client, workspaceID types.String) *client.Client {
	if workspaceID.IsNull() || workspaceID.IsUnknown() {
		return c
	}
	if v := workspaceID.ValueString(); v != "" {
		return c.WithWorkspaceID(v)
	}
	return c
}

// firstNonEmpty returns the first non-empty string. Useful for decoding
// workspace identifiers from APIs that return either `workspace_id` or the
// legacy `tenant_id` key.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// finalizeWorkspaceID reconciles the Terraform state value of workspace_id with
// the value returned by the API (see reconcileWorkspaceID) and, when the state
// value is still null or unknown afterwards, falls back to the workspace the
// client is operating in (provider-level configuration or env). This guarantees
// workspace_id is never left unknown after apply.
func finalizeWorkspaceID(state *types.String, c *client.Client, apiValue string, diags *diag.Diagnostics) {
	reconcileWorkspaceID(state, apiValue, diags)
	if state.IsNull() || state.IsUnknown() {
		if c != nil && c.WorkspaceID != "" {
			*state = types.StringValue(c.WorkspaceID)
		} else {
			*state = types.StringNull()
		}
	}
}

// reconcileWorkspaceID reconciles the Terraform state value of workspace_id with the
// value returned by the API:
//   - If apiValue is empty (the API did not return a workspace_id), the state is
//     set to null when it is currently null or unknown, and left unchanged otherwise.
//     No mismatch warning is emitted because the API simply has nothing to compare.
//   - If apiValue is non-empty and the state value is null or unknown, it is
//     populated from the API response.
//   - If apiValue is non-empty and the state value was explicitly set by the user
//     but differs from the API response, a warning is added and the user-provided
//     value is kept in state.
func reconcileWorkspaceID(state *types.String, apiValue string, diags *diag.Diagnostics) {
	if apiValue == "" {
		// The API did not provide a workspace_id.  Normalise null/unknown to
		// null so we never leave an unknown value in state.
		if state.IsNull() || state.IsUnknown() {
			*state = types.StringNull()
		}
		return
	}
	if state.IsNull() || state.IsUnknown() {
		*state = types.StringValue(apiValue)
		return
	}
	if state.ValueString() != apiValue {
		diags.AddWarning(
			"Workspace ID mismatch",
			fmt.Sprintf("The configured workspace_id %q does not match the API-returned workspace_id %q. Using the configured value.", state.ValueString(), apiValue),
		)
	}
}
