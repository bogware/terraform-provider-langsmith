// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccAlertRuleResource_basic sets up a lookout on the project and waits
// for trouble. Marshal Dillon never let a disturbance go unnoticed, and
// neither should your alert rules — if latency crosses the line, you'll know.
//
// It covers create (#93: the API echoes extra fields on every action), an
// update that edits the rule and drops one of two actions (PATCH needs
// rule.id and answers with only a status message), and import. The action
// configs use the per-target keys established against the live API in #86.
// Neither action delivers anything unless the rule actually fires: the
// webhook points at example.com and the Slack IDs are placeholders.
func TestAccAlertRuleResource_basic(t *testing.T) {
	rName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	const webhook = `{
      target = "webhook"
      config = jsonencode({
        url          = "https://example.com/langsmith-alert"
        project_name = langsmith_project.test.name
        headers      = jsonencode({})
        body         = jsonencode({ text = "latency alert" })
      })
    }`
	const slack = `{
      target = "slack"
      config = jsonencode({
        team_id    = "T00000000"
        channel_id = "C00000000"
      })
    }`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAlertRuleConfig(rName, 5000, webhook+", "+slack),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_alert_rule.test", "id"),
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "name", fmt.Sprintf("tf-acc-test-alert-%s", rName)),
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "type", "threshold"),
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "threshold", "5000"),
				),
			},
			{
				Config: testAccAlertRuleConfig(rName, 8000, webhook),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_alert_rule.test", "threshold", "8000"),
					resource.TestCheckResourceAttrWith("langsmith_alert_rule.test", "actions", func(v string) error {
						if strings.Contains(v, "slack") {
							return fmt.Errorf("slack action should have been removed, actions = %s", v)
						}
						return nil
					}),
				),
			},
			{
				ResourceName:      "langsmith_alert_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["langsmith_alert_rule.test"]
					if !ok {
						return "", fmt.Errorf("langsmith_alert_rule.test not found in state")
					}
					return rs.Primary.Attributes["session_id"] + "/" + rs.Primary.ID, nil
				},
			},
		},
	})
}

func testAccAlertRuleConfig(rName string, threshold int, actions string) string {
	return fmt.Sprintf(`
resource "langsmith_project" "test" {
  name = "tf-acc-test-alert-%[1]s"
}

resource "langsmith_alert_rule" "test" {
  session_id     = langsmith_project.test.id
  name           = "tf-acc-test-alert-%[1]s"
  description    = "Test alert rule"
  type           = "threshold"
  aggregation    = "avg"
  attribute      = "latency"
  operator       = "gte"
  window_minutes = 5
  threshold      = %[2]d

  actions = jsonencode([
    %[3]s
  ])
}
`, rName, threshold, actions)
}

// TestAlertRuleActionsValue covers how the API's actions echo becomes state.
// The API adds id, alert_rule_id, created_at and updated_at to every action
// (and on Cloud, the is_suspended / suspended_* fields too), and copying that
// into state failed every create with "Provider produced inconsistent result
// after apply" (#93).
func TestAlertRuleActionsValue(t *testing.T) {
	const cfg = `[{"config":"{\"headers\":\"{}\",\"url\":\"https://example.com/hook\"}","target":"webhook"}]`
	const echoed = `[{"alert_rule_id":"efaffa03","config":"{\"headers\":\"{}\",\"url\":\"https://example.com/hook\"}",` +
		`"created_at":"2026-10-26T20:15:33.703859212Z","id":"94618039","target":"webhook",` +
		`"updated_at":"2026-10-26T20:15:33.703859212Z"}]`

	slack := `{"config":"{\"channel\":\"#alerts\"}","target":"slack"}`
	hook := `{"config":"{\"url\":\"https://example.com/hook\"}","target":"webhook"}`
	withIDs := func(action, id string) string {
		return `{"id":"` + id + `","alert_rule_id":"r1","created_at":"t","updated_at":"t",` + action[1:]
	}

	cases := []struct {
		name  string
		raw   string
		saved types.String
		want  types.String
	}{
		{
			name:  "server keys are ignored and the configured value is kept",
			raw:   echoed,
			saved: types.StringValue(cfg),
			want:  types.StringValue(cfg),
		},
		{
			name:  "configured value is kept byte-for-byte, even when not compact",
			raw:   echoed,
			saved: types.StringValue("[\n  " + cfg[1:len(cfg)-1] + "\n]"),
			want:  types.StringValue("[\n  " + cfg[1:len(cfg)-1] + "\n]"),
		},
		{
			name:  "actions returned in a different order still match",
			raw:   `[` + withIDs(slack, "a") + `,` + withIDs(hook, "b") + `]`,
			saved: types.StringValue(`[` + hook + `,` + slack + `]`),
			want:  types.StringValue(`[` + hook + `,` + slack + `]`),
		},
		{
			name:  "import has no saved value, so the response is stripped",
			raw:   echoed,
			saved: types.StringNull(),
			want:  types.StringValue(cfg),
		},
		{
			name:  "a config changed outside Terraform surfaces as drift",
			raw:   `[` + withIDs(`{"config":"{\"url\":\"https://other.example.com\"}","target":"webhook"}`, "a") + `]`,
			saved: types.StringValue(`[` + hook + `]`),
			want:  types.StringValue(`[{"config":"{\"url\":\"https://other.example.com\"}","target":"webhook"}]`),
		},
		{
			name:  "an action added outside Terraform surfaces as drift",
			raw:   `[` + withIDs(hook, "a") + `,` + withIDs(slack, "b") + `]`,
			saved: types.StringValue(`[` + hook + `]`),
			want:  types.StringValue(`[` + hook + `,` + slack + `]`),
		},
		{
			name:  "duplicate saved actions need distinct matches",
			raw:   `[` + withIDs(hook, "a") + `,` + withIDs(slack, "b") + `]`,
			saved: types.StringValue(`[` + hook + `,` + hook + `]`),
			want:  types.StringValue(`[` + hook + `,` + slack + `]`),
		},
		{
			name:  "state written by 1.2.1 with server keys is repaired to the stripped form",
			raw:   echoed,
			saved: types.StringValue(echoed),
			want:  types.StringValue(cfg),
		},
		{
			// Captured from LangSmith Cloud: GET re-sorts the keys inside
			// config, while create echoes the string exactly as sent.
			name: "a hand-written unsorted config matches the re-sorted GET config",
			raw: `[{"id":"a","alert_rule_id":"r1","target":"webhook",` +
				`"config":"{\"body\":\"{}\",\"headers\":\"{}\",\"url\":\"https://example.com/hook\"}"}]`,
			saved: types.StringValue(`[{"target":"webhook","config":"{\"url\":\"https://example.com/hook\",\"headers\":\"{}\",\"body\":\"{}\"}"}]`),
			want:  types.StringValue(`[{"target":"webhook","config":"{\"url\":\"https://example.com/hook\",\"headers\":\"{}\",\"body\":\"{}\"}"}]`),
		},
		{
			name: "a suspended action is not drift",
			raw: `[{"id":"a","alert_rule_id":"r1","created_at":"t","updated_at":"t","is_suspended":true,` +
				`"suspended_at":"t","suspended_status_code":410,"suspended_reason":"gone",` + hook[1:] + `]`,
			saved: types.StringValue(`[` + hook + `]`),
			want:  types.StringValue(`[` + hook + `]`),
		},
		{
			name: "import keeps only target and config, whatever else the server adds",
			raw: `[{"id":"a","alert_rule_id":"r1","created_at":"t","updated_at":"t","is_suspended":false,` +
				`"suspended_at":null,"suspended_status_code":null,"suspended_reason":null,"some_future_field":1,` + hook[1:] + `]`,
			saved: types.StringNull(),
			want:  types.StringValue(`[` + hook + `]`),
		},
		{
			name:  "a different config value is still drift after decoding",
			raw:   `[` + withIDs(`{"config":"{\"url\":\"https://example.com/hook\",\"x\":1}","target":"webhook"}`, "a") + `]`,
			saved: types.StringValue(`[` + hook + `]`),
			want:  types.StringValue(`[{"config":"{\"url\":\"https://example.com/hook\",\"x\":1}","target":"webhook"}]`),
		},
		{
			name:  "a response without actions keeps the saved value",
			raw:   `null`,
			saved: types.StringValue(cfg),
			want:  types.StringValue(cfg),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := alertRuleActionsValue([]byte(tc.raw), tc.saved)
			if !got.Equal(tc.want) {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
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
			// LangSmith answers this with a bare 200 and creates nothing.
			name:        "webhook headers as an object is rejected",
			actions:     `[{"target":"webhook","config":"{\"url\":\"https://example.com/hook\",\"headers\":{},\"body\":\"{}\"}"}]`,
			wantErr:     true,
			wantSummary: "Webhook headers must be a JSON-encoded string",
		},
		{
			name:        "webhook body as an object is rejected",
			actions:     `[{"target":"webhook","config":"{\"url\":\"https://example.com/hook\",\"headers\":\"{}\",\"body\":{\"text\":\"x\"}}"}]`,
			wantErr:     true,
			wantSummary: "Webhook body must be a JSON-encoded string",
		},
		{
			name:    "webhook headers and body as JSON-encoded strings are accepted",
			actions: `[{"target":"webhook","config":"{\"url\":\"https://example.com/hook\",\"headers\":\"{}\",\"body\":\"{}\"}"}]`,
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
