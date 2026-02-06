resource "langsmith_bulk_export" "example" {
  bulk_export_destination_id = langsmith_bulk_export_destination.example.id
  session_id                 = langsmith_project.example.id
  start_time                 = "2024-01-01T00:00:00Z"
}
