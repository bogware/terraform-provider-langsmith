// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
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
