data "langsmith_run_rule_logs" "example" {
  rule_id = langsmith_run_rule.example.id
}

output "last_applied" {
  value = data.langsmith_run_rule_logs.example.last_applied
}

output "log_count" {
  value = length(data.langsmith_run_rule_logs.example.logs)
}
