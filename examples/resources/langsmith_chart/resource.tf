# Series use LangSmith's V2 chart format: metric_definition says what to
# measure and filter_definition says which runs to include. The V1 keys
# (metric, filters, feedback_key) are no longer accepted.
resource "langsmith_chart" "example" {
  title      = "Run Latency"
  chart_type = "line"
  section_id = langsmith_chart_section.example.id
  series = jsonencode([
    {
      name = "p50 latency"
      metric_definition = {
        type   = "percentile"
        field  = "latency_seconds"
        params = { p = 0.5 }
      }
      filter_definition = {
        source_type = "tracing_project"
        project_ids = [langsmith_project.example.id]
      }
    },
    {
      name = "p99 latency"
      metric_definition = {
        type   = "percentile"
        field  = "latency_seconds"
        params = { p = 0.99 }
      }
      filter_definition = {
        source_type = "tracing_project"
        project_ids = [langsmith_project.example.id]
      }
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
      name              = "run count"
      metric_definition = { type = "count" }
      filter_definition = {
        source_type = "tracing_project"
        project_ids = [langsmith_project.example.id]
      }
    }
  ])
}

# Average token usage, split by run type.
resource "langsmith_chart" "tokens_by_run_type" {
  title      = "Average Tokens by Run Type"
  chart_type = "bar"
  section_id = langsmith_chart_section.example.id
  series = jsonencode([
    {
      name                 = "avg total tokens"
      metric_definition    = { type = "avg", field = "total_tokens" }
      group_by_definitions = [{ attribute = "run_type" }]
      filter_definition = {
        source_type = "tracing_project"
        project_ids = [langsmith_project.example.id]
      }
    }
  ])
}
