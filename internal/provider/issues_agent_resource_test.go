// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccIssuesAgentResource_basic exercises the Beta issues-agent endpoints.
// It needs an existing tracing project (session) with the Issues Agent feature
// enabled; set LANGSMITH_TEST_ISSUES_AGENT_SESSION_ID to the project's UUID to
// enable. Only one issues agent can exist per session, so the test session
// must not already have one.
func TestAccIssuesAgentResource_basic(t *testing.T) {
	sessionID := os.Getenv("LANGSMITH_TEST_ISSUES_AGENT_SESSION_ID")
	if sessionID == "" {
		t.Skip("Set LANGSMITH_TEST_ISSUES_AGENT_SESSION_ID to a tracing project UUID to enable (Beta feature)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "langsmith_issues_agent" "test" {
  session_id = %[1]q
  priorities = ["P0", "P1"]
}
`, sessionID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_issues_agent.test", "id"),
					resource.TestCheckResourceAttr("langsmith_issues_agent.test", "session_id", sessionID),
					resource.TestCheckResourceAttr("langsmith_issues_agent.test", "priorities.#", "2"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "langsmith_issues_agent" "test" {
  session_id        = %[1]q
  priorities        = ["P0", "P1"]
  user_instructions = "Focus on tool-call failures."
  overview          = "# Overview\n\nTest agent overview."
}
`, sessionID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_issues_agent.test", "user_instructions", "Focus on tool-call failures."),
					resource.TestCheckResourceAttr("langsmith_issues_agent.test", "overview", "# Overview\n\nTest agent overview."),
					// Saving the overview creates the backing Prompt Hub repo.
					resource.TestCheckResourceAttrSet("langsmith_issues_agent.test", "session_agent_overview_repo_id"),
				),
			},
			{
				// overview is write-only, so no GET can refresh it: changing it
				// must still plan and apply cleanly, and the new content must
				// survive the refresh that follows the apply.
				Config: fmt.Sprintf(`
resource "langsmith_issues_agent" "test" {
  session_id        = %[1]q
  priorities        = ["P0", "P1"]
  user_instructions = "Focus on tool-call failures."
  overview          = "# Overview\n\nUpdated agent overview."
}
`, sessionID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_issues_agent.test", "overview", "# Overview\n\nUpdated agent overview."),
				),
			},
			{
				// ImportStateVerify is off: overview is never returned by the
				// API, so an imported agent has it null while state has content.
				ResourceName:      "langsmith_issues_agent.test",
				ImportState:       true,
				ImportStateId:     sessionID,
				ImportStateVerify: false,
			},
		},
	})
}
