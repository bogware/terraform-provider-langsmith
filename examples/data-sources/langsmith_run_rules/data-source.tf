data "langsmith_run_rules" "all" {}

data "langsmith_run_rules" "session_rules" {
  type = "session"
}

output "rule_names" {
  value = [for r in data.langsmith_run_rules.all.rules : r.display_name]
}
