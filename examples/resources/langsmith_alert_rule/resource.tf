resource "langsmith_alert_rule" "example" {
  session_id     = langsmith_project.example.id
  name           = "High Latency Alert"
  description    = "Alert when p50 latency exceeds 5 seconds"
  type           = "threshold"
  aggregation    = "avg"
  attribute      = "latency"
  operator       = "gte"
  threshold      = 5000
  window_minutes = 60
  # At least one action is required; a rule with an empty array is rejected.
  # Note the double encoding: config is itself a JSON-encoded string, not a
  # nested object, because its keys differ per target. A webhook's headers and
  # body are JSON-encoded strings as well. See the actions attribute below for
  # the keys each target (webhook, slack, pagerduty, dynatrace) requires.
  actions = jsonencode([
    {
      target = "webhook"
      config = jsonencode({
        url          = "https://example.com/langsmith-alert"
        project_name = langsmith_project.example.name
        headers      = jsonencode({ "Content-Type" = "application/json" })
        body         = jsonencode({ text = "p50 latency is over 5 seconds" })
      })
    },
    {
      target = "pagerduty"
      config = jsonencode({
        integration_key = var.pagerduty_integration_key
        severity        = "critical"
      })
    }
  ])
}
