// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccOrgMemberResource_basic verifies organization member management.
//
// Skipped: requires a second user and organization:manage permission.
func TestAccOrgMemberResource_basic(t *testing.T) {
	t.Skip("Requires a second user and organization:manage permission")
}
