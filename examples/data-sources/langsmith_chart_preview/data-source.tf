data "langsmith_chart_preview" "example" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-02T00:00:00Z"
  stride     = jsonencode({ hours = 1 })

  series = jsonencode([
    {
      # Each preview series requires an id (any UUID — it has no persistence).
      id     = "00000000-0000-0000-0000-000000000001"
      name   = "Run Count"
      metric = "run_count"

      # Workspace-scoped previews require a `session` (project) filter on each
      # series, otherwise the API returns 422.
      filters = {
        session = [data.langsmith_project.example.id]
      }
    }
  ])
}

output "preview_data" {
  value = jsondecode(data.langsmith_chart_preview.example.data)
}
