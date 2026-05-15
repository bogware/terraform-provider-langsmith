data "langsmith_audit_log" "last_hour" {
  start_time = "2026-05-14T12:00:00Z"
  end_time   = "2026-05-14T13:00:00Z"
  limit      = 100
}
