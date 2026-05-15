data "langsmith_chart_preview" "example" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  stride     = jsonencode({ hours = 1 })

  series = jsonencode([
    {
      name   = "Run Count"
      metric = "run_count"
    }
  ])
}

output "preview_data" {
  value = jsondecode(data.langsmith_chart_preview.example.data)
}
