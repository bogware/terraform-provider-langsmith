data "langsmith_org_chart_preview" "example" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  stride     = jsonencode({ hours = 1 })

  series = jsonencode([
    {
      # Each preview series requires an id (any UUID — it has no persistence).
      id     = "00000000-0000-0000-0000-000000000001"
      name   = "Run Count"
      metric = "run_count"
    }
  ])
}
