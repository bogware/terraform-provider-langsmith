data "langsmith_evaluators" "all" {}

data "langsmith_evaluators" "llm" {
  type = "llm"
}

output "evaluator_names" {
  value = [for e in data.langsmith_evaluators.all.evaluators : e.name]
}
