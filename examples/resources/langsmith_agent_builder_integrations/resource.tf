# Singleton workspace-level settings: disable Agent Builder integrations by
# default, then allow-list specific integrations.
resource "langsmith_agent_builder_integrations" "this" {
  integrations_enabled_by_default = false

  integration_overrides = [
    {
      integration_key = "slack"
      is_enabled      = true
    },
    {
      integration_key = "github"
      is_enabled      = true
    },
  ]
}
