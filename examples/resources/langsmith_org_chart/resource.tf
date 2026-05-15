resource "langsmith_org_chart" "example" {
  title      = "Run Latency (Org)"
  chart_type = "line"
  section_id = langsmith_org_chart_section.example.id
  series = jsonencode([
    {
      name   = "p50 latency"
      metric = "latency_p50"
    }
  ])
}
