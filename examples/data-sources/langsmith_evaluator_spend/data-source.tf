data "langsmith_evaluator_spend" "current" {}

output "evaluator_spend" {
  value = jsondecode(data.langsmith_evaluator_spend.current.spend_json)
}
