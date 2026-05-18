resource "langsmith_feedback_ingest_token" "thumbs" {
  run_id       = "00000000-0000-0000-0000-000000000000"
  feedback_key = "user_thumbs"
  expires_at   = "2026-06-01T00:00:00Z"
}

output "ingest_url" {
  value     = langsmith_feedback_ingest_token.thumbs.url
  sensitive = true
}
