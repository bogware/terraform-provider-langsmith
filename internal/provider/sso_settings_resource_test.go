// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccSSOSettingsResource_basic swings the saloon doors open with a
// SAML metadata URL and checks that single sign-on is properly posted
// on the notice board. One way in, one way out — just like the Long Branch.
//
// Skipped: requires organization:manage permission (enterprise tier).
func TestAccSSOSettingsResource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission (enterprise tier)")
}
