resource "langsmith_gateway_policy" "monthly_cap" {
  name        = "monthly-spend-cap"
  description = "Cap production org spend to $1k/month"
  policy_type = "spend_cap"
  action      = "block"
  enabled     = true
  priority    = 10

  config = jsonencode({
    amount_usd = 1000
    window     = "month"
  })

  subject_matchers = [
    {
      key   = "workspace_id"
      value = langsmith_workspace.production.id
    },
  ]
}
