resource "langsmith_run_rule" "example" {
  display_name  = "sample-10-percent"
  sampling_rate = 0.1
  session_id    = langsmith_project.example.id
  is_enabled    = true
}
