// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

func TestParseGatewaySubjectMatchersJSON(t *testing.T) {
	t.Parallel()
	got, err := parseGatewaySubjectMatchersJSON(`[{"key":"organization_id","value":"x"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "organization_id" || got[0].Value != "x" {
		t.Fatalf("unexpected: %#v", got)
	}
	if _, err := parseGatewaySubjectMatchersJSON(`[{"value":"x"}]`); err == nil {
		t.Fatal("expected error for missing key")
	}
}

// testAccPreCheckGatewayPoliciesAPI skips when the gateway policy API is not
// available (403 when LLM Gateway is disabled or the key lacks org read).
func testAccPreCheckGatewayPoliciesAPI(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	apiURL := os.Getenv("LANGSMITH_API_URL")
	if apiURL == "" {
		apiURL = "https://api.smith.langchain.com"
	}
	if os.Getenv("LANGSMITH_ORGANIZATION_ID") == "" {
		t.Skip("LANGSMITH_ORGANIZATION_ID must be set for gateway policy acceptance tests")
	}
	c := client.NewClient(
		apiURL,
		os.Getenv("LANGSMITH_API_KEY"),
		os.Getenv("LANGSMITH_TENANT_ID"),
		os.Getenv("LANGSMITH_ORGANIZATION_ID"),
		"terraform-provider-langsmith-acc-gateway-precheck",
	)
	var probe []map[string]any
	err := c.Get(ctx, "/v1/platform/gateway-policies", nil, &probe)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusUnauthorized) {
			t.Skipf("GET /v1/platform/gateway-policies returned %d; skipping gateway policy acceptance test (gateway disabled or insufficient permissions)", apiErr.StatusCode)
		}
		t.Fatalf("gateway policies API pre-check: %v", err)
	}
}

func TestAccGatewayPolicyResource_basic(t *testing.T) {
	rName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	orgID := os.Getenv("LANGSMITH_ORGANIZATION_ID")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckGatewayPoliciesAPI(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_gateway_policy" "test" {
  name        = "tf-acc-gw-%s"
  description = "Terraform acceptance"
  policy_type = "guard"
  action      = "block"
  subject_matchers = jsonencode([
    { key = "organization_id", value = %q }
  ])
  config = jsonencode({
    version = 1
    detect = {
      pii     = false
      secrets = false
    }
  })
  enabled  = true
  priority = 10
}
`, rName, orgID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_gateway_policy.test", "id"),
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "name", fmt.Sprintf("tf-acc-gw-%s", rName)),
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "policy_type", "guard"),
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "enabled", "true"),
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "priority", "10"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "langsmith_gateway_policy" "test" {
  name        = "tf-acc-gw-%s"
  description = "Terraform acceptance"
  policy_type = "guard"
  action      = "block"
  subject_matchers = jsonencode([
    { key = "organization_id", value = %q }
  ])
  config = jsonencode({
    version = 1
    detect = {
      pii     = false
      secrets = true
    }
  })
  enabled  = false
  priority = 20
}
`, rName, orgID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "enabled", "false"),
					resource.TestCheckResourceAttr("langsmith_gateway_policy.test", "priority", "20"),
				),
			},
			{
				ResourceName:      "langsmith_gateway_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
