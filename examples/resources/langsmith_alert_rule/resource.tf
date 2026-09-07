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
  actions = jsonencode([
    {
      target = "webhook"
      config = {
        url = "https://example.com/langsmith-alert"
      }
    }
  ])
}
