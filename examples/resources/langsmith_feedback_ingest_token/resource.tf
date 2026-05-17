resource "langsmith_feedback_ingest_token" "external_rater" {
  run_id       = "00000000-0000-4000-8000-000000000001"
  feedback_key = "human_notes"
  # Optional relative lifetime (OpenAPI TimedeltaInput)
  # expires_in = jsonencode({ days = 7 })
}
