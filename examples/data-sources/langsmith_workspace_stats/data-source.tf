data "langsmith_workspace_stats" "current" {}

output "dataset_count" {
  value = data.langsmith_workspace_stats.current.dataset_count
}

output "tracing_project_count" {
  value = data.langsmith_workspace_stats.current.tracer_session_count
}
