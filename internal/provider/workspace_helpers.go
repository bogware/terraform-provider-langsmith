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

// reconcileWorkspaceID reconciles the Terraform state value of workspace_id with the
// value returned by the API:
//   - If the state value is null or unknown it is populated from the API response.
//   - If the state value was explicitly set by the user and matches the API
//     response, nothing happens.
//   - If the state value was explicitly set but differs from the API response, a
//     warning is added and the user-provided value is kept in state.
func reconcileWorkspaceID(state *types.String, apiValue string, diags *diag.Diagnostics) {
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
