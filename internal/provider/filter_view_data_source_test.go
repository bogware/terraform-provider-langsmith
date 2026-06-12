// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestAccFilterViewDataSource_basic would create a filter view in a project
// and read it back through the data source.
//
// Skipped: the filter view API intermittently returns 500 errors (see
// TestAccFilterViewResource_basic).
func TestAccFilterViewDataSource_basic(t *testing.T) {
	t.Skip("Filter view API intermittently returns 500 errors; requires further investigation")
}
