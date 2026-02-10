resource "langsmith_filter_view" "example" {
  session_id    = langsmith_project.example.id
  display_name  = "Error Runs"
  filter_string = "eq(status, \"error\")"
}
