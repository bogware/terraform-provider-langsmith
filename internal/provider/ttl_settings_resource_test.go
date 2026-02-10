// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccTTLSettingsResource_basic checks TTL settings management.
//
// Skipped: requires organization:manage permission (enterprise tier).
func TestAccTTLSettingsResource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
}
