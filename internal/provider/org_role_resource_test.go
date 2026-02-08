// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccOrgRoleResource_basic pins a badge on a new role and makes sure
// it carries the right authority. In Dodge City every deputy needs clear
// jurisdiction — same goes for organization roles in LangSmith.
//
// Skipped: requires organization:manage permission (enterprise tier).
func TestAccOrgRoleResource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
}
