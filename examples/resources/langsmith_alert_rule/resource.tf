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
  actions        = jsonencode([])
}
