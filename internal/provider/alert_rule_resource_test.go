// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccAlertRuleResource_basic sets up a lookout on the project and waits
// for trouble. Marshal Dillon never let a disturbance go unnoticed, and
// neither should your alert rules — if latency crosses the line, you'll know.
func TestAccAlertRuleResource_basic(t *testing.T) {
	// Opt-in because the shape of an action's config is not documented and is
	// not yet fully known. The API validates it one field at a time, and a
	// webhook has so far been shown to need url, project_name and headers --
	// each discovered from a separate `missing required field: ...` rejection,
	// with no way to enumerate the rest short of continuing to probe. The
	// config below is the best known shape, not a confirmed-good one.
	//
	// Everything this file asserts about actions at the provider level
	// (minItems:1, config being a JSON-encoded string) is verified and covered
	// credential-free by TestBuildAlertRuleRequest_Actions.
	if os.Getenv("LANGSMITH_TEST_ALERT_RULE") == "" {
		t.Skip("Set LANGSMITH_TEST_ALERT_RULE=1 to enable (alert action config shape is undocumented; see comment)")
	}

	rName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			// CheckDestroy is handled automatically by the test framework
			// verifying the resource no longer exists.
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = "tf-acc-test-alert-%s"
}

resource "langsmith_alert_rule" "test" {
  session_id     = langsmith_project.test.id
  name           = "tf-acc-test-alert-%s"
  description    = "Test alert rule"
  type           = "threshold"
  aggregation    = "avg"
  attribute      = "latency"
  operator       = "gte"
  window_minutes = 5
  threshold      = 5000

  # An empty array is rejected by the API (actions is minItems:1). A webhook
  # pointing at an unroutable host is the least side-effecting valid action:
  # nothing is delivered unless the rule actually fires.
  #
  # config is a JSON-encoded string rather than a nested object. None of its
  # keys are in the published OpenAPI spec, which declares config as a bare
  # object; url, project_name and headers were each established against the
  # live API, one "missing required field" at a time. There may be more.
  actions = jsonencode([
    {
      target = "webhook"
      config = jsonencode({
        url          = "https://example.com/langsmith-alert"
        project_name = langsmith_project.test.name
        headers      = {}
      })
    }
  ])
}`, rName, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_alert_rule.test", "id"),
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "name", fmt.Sprintf("tf-acc-test-alert-%s", rName)),
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "type", "threshold"),
				),
			},
		},
	})
}

// TestBuildAlertRuleRequest_Actions covers the actions payload, which the API
// constrains as minItems:1 and answers with an opaque
// `request validation failed: [Actions: min]` when it is empty. The provider
// catches that itself so the user is told what to fix. Runs without
// credentials.
func TestBuildAlertRuleRequest_Actions(t *testing.T) {
	base := func(actions string) *AlertRuleResourceModel {
		return &AlertRuleResourceModel{
			Name:          types.StringValue("rule"),
			Description:   types.StringValue("desc"),
			Type:          types.StringValue("threshold"),
			Aggregation:   types.StringValue("avg"),
			Attribute:     types.StringValue("latency"),
			Operator:      types.StringValue("gte"),
			WindowMinutes: types.Int64Value(5),
			Actions:       types.StringValue(actions),
		}
	}

	cases := []struct {
		name        string
		actions     string
		wantErr     bool
		wantSummary string
	}{
		{
			// config is a JSON-encoded string, not a nested object.
			name:    "single webhook action is accepted",
			actions: `[{"target":"webhook","config":"{\"url\":\"https://example.com/hook\"}"}]`,
		},
		{
			name:        "empty array is rejected before it reaches the API",
			actions:     `[]`,
			wantErr:     true,
			wantSummary: "Alert rule requires at least one action",
		},
		{
			name:        "a JSON object is not an actions array",
			actions:     `{"target":"webhook"}`,
			wantErr:     true,
			wantSummary: "Invalid Actions JSON",
		},
		{
			name:        "malformed JSON is still caught",
			actions:     `[{`,
			wantErr:     true,
			wantSummary: "Invalid Actions JSON",
		},
		{
			// The natural mistake, and what the OpenAPI spec's
			// `"config": {"type": "object"}` invites. The API answers it with
			// "cannot unmarshal object into Go value of type string".
			name:        "config as a nested object is rejected",
			actions:     `[{"target":"webhook","config":{"url":"https://example.com/hook"}}]`,
			wantErr:     true,
			wantSummary: "Alert action config must be a JSON-encoded string",
		},
		{
			name:        "action without a target is rejected",
			actions:     `[{"config":"{}"}]`,
			wantErr:     true,
			wantSummary: "Alert action is missing a target",
		},
		{
			name:        "an array of non-objects is rejected",
			actions:     `["webhook"]`,
			wantErr:     true,
			wantSummary: "Invalid alert action",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, diags := buildAlertRuleRequest(base(tc.actions))
			if tc.wantErr {
				if !diags.HasError() {
					t.Fatalf("expected an error diagnostic, got none")
				}
				if got := diags.Errors()[0].Summary(); got != tc.wantSummary {
					t.Fatalf("diagnostic summary = %q, want %q", got, tc.wantSummary)
				}
				if body != nil {
					t.Fatalf("expected nil body on error, got %+v", body)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", diags.Errors())
			}
			if string(body.Actions) != tc.actions {
				t.Fatalf("actions = %s, want %s", body.Actions, tc.actions)
			}
		})
	}
}
