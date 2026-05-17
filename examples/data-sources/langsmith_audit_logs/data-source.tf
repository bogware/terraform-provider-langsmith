data "langsmith_audit_logs" "recent" {
  start_time = "2026-01-01T00:00:00Z"
  end_time   = "2026-01-31T23:59:59Z"
  operations = ["create_api_key", "delete_api_key"]
  limit      = 50
}
