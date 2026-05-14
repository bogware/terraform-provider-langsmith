resource "langsmith_org_chart" "example" {
  title      = "Run volume"
  chart_type = "line"
  section_id = langsmith_org_chart_section.example.id
  series = jsonencode([
    {
      name   = "Run Count"
      metric = "run_count"
    }
  ])
}
