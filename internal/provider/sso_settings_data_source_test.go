// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccSSOSettingsDataSource_basic would read the current organization's
// SSO settings through the data source.
//
// Skipped: requires organization:manage permission and an existing SSO
// configuration (enterprise tier), matching TestAccSSOSettingsResource_basic.
func TestAccSSOSettingsDataSource_basic(t *testing.T) {
	t.Skip("Requires organization:manage permission and an existing SSO configuration (enterprise tier)")
}
