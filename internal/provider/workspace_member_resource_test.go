// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccWorkspaceMemberResource_basic invites a new hand to the outfit and
// makes sure they're on the roster. Even Miss Kitty had to vouch for her
// people — this test ensures workspace membership is properly recorded.
//
// Skipped: requires a second user and team/enterprise tier.
func TestAccWorkspaceMemberResource_basic(t *testing.T) {
	t.Skip("Requires a second user and team/enterprise tier to add workspace members")
}
