// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// effectiveClient returns a shallow copy of c with TenantID set to the
// resource-level override when the caller has supplied a non-empty tenant_id.
// If tenantID is null, unknown, or the empty string the original client is
// returned unchanged, so the provider-level TenantID (or none) is used.
func effectiveClient(c *client.Client, tenantID types.String) *client.Client {
	if tenantID.IsNull() || tenantID.IsUnknown() {
		return c
	}
	if v := tenantID.ValueString(); v != "" {
		return c.WithTenantID(v)
	}
	return c
}

// reconcileTenantID reconciles the Terraform state value of tenant_id with the
// value returned by the API:
//   - If the state value is null or unknown it is populated from the API response.
//   - If the state value was explicitly set by the user and matches the API
//     response, nothing happens.
//   - If the state value was explicitly set but differs from the API response, a
//     warning is added and the user-provided value is kept in state.
func reconcileTenantID(state *types.String, apiValue string, diags *diag.Diagnostics) {
	if state.IsNull() || state.IsUnknown() {
		*state = types.StringValue(apiValue)
		return
	}
	if state.ValueString() != apiValue {
		diags.AddWarning(
			"Tenant ID mismatch",
			fmt.Sprintf("The configured tenant_id %q does not match the API-returned tenant_id %q. Using the configured value.", state.ValueString(), apiValue),
		)
	}
}
