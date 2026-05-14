// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is the law of the land for acceptance tests —
// Protocol 6, the only authority recognized in this territory.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"langsmith": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck makes sure you've brought your credentials before riding into
// test territory. No API key, no entry — the marshal's orders.
func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("LANGSMITH_API_KEY"); v == "" {
		t.Fatal("LANGSMITH_API_KEY must be set for acceptance tests")
	}
}

// testAccPreCheckOrgCharts requires LANGSMITH_ORGANIZATION_ID for org chart API calls
// (X-Organization-Id), in addition to the usual API key.
func testAccPreCheckOrgCharts(t *testing.T) {
	testAccPreCheck(t)
	if os.Getenv("LANGSMITH_ORGANIZATION_ID") == "" {
		t.Skip("LANGSMITH_ORGANIZATION_ID must be set for org chart acceptance tests")
	}
}
