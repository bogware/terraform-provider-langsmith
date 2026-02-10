// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccFilterViewResource_basic verifies filter view CRUD within a project.
//
// Skipped: the filter view API intermittently returns 500 errors.
func TestAccFilterViewResource_basic(t *testing.T) {
	t.Skip("Filter view API intermittently returns 500 errors; requires further investigation")
}
