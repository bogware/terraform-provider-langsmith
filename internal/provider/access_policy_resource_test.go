// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccAccessPolicyResource_basic verifies creation and deletion of
// an ABAC access policy.
//
// Skipped: requires enterprise tier with ABAC enabled.
func TestAccAccessPolicyResource_basic(t *testing.T) {
	t.Skip("Requires enterprise tier with ABAC enabled")
}
