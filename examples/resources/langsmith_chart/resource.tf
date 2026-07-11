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

# A KPI chart renders a single aggregate value rather than a time series.
resource "langsmith_chart" "run_count_kpi" {
  title      = "Total Runs"
  chart_type = "kpi"
  section_id = langsmith_chart_section.example.id
  series = jsonencode([
    {
      name   = "run count"
      metric = "run_count"
    }
  ])
}
