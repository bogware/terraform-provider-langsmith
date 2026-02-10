resource "langsmith_chart" "example" {
  title      = "Run Latency"
  chart_type = "line"
  section_id = langsmith_chart_section.example.id
  series = jsonencode([
    {
      name   = "p50 latency"
      metric = "latency_p50"
    }
  ])
}
