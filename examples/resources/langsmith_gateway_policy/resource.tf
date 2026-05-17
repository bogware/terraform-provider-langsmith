# LangSmith LLM Gateway policies are organization-scoped. Set
# LANGSMITH_ORGANIZATION_ID (or provider `organization_id`) and use an API key
# with gateway permissions. This example uses a lightweight guard policy scoped
# to the organization; adjust `subject_matchers` to target a workspace (`workspace_id`)
# or other subjects per the OpenAPI `gateway_policies` schemas.

resource "langsmith_gateway_policy" "example" {
  name        = "example-guard"
  description = "Example guard policy managed by Terraform"
  policy_type = "guard"
  action      = "block"
  subject_matchers = jsonencode([
    { key = "organization_id", value = "<organization-uuid>" }
  ])
  config = jsonencode({
    version = 1
    detect = {
      pii     = true
      secrets = true
    }
  })
  enabled  = true
  priority = 0
}
