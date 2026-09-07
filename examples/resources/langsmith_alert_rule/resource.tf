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
  # Note the double encoding: `config` is itself a JSON-encoded string, not a
  # nested object, because it carries a different shape per target.
  actions = jsonencode([
    {
      target = "webhook"
      config = jsonencode({
        url = "https://example.com/langsmith-alert"
      })
    }
  ])
}
