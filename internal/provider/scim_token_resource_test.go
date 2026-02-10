// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccSCIMTokenResource_basic verifies SCIM token lifecycle.
//
// Skipped: requires enterprise tier with SCIM enabled.
func TestAccSCIMTokenResource_basic(t *testing.T) {
	t.Skip("Requires enterprise tier with SCIM enabled")
}
