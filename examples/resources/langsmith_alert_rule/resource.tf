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
  # nested object, because its keys differ per target.
  #
  # Those keys are not documented by LangSmith. A webhook needs at least url,
  # project_name and headers; if the API reports a missing field, add it here.
  actions = jsonencode([
    {
      target = "webhook"
      config = jsonencode({
        url          = "https://example.com/langsmith-alert"
        project_name = langsmith_project.example.name
        headers      = {}
      })
    }
  ])
}
