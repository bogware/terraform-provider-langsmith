resource "langsmith_run_share" "demo_trace" {
  run_id = "00000000-0000-0000-0000-000000000000"
}

output "run_share_token" {
  value = langsmith_run_share.demo_trace.share_token
}
