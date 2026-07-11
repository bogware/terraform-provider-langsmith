# Feedback formulas scoped to a dataset.
data "langsmith_feedback_formulas" "by_dataset" {
  dataset_id = langsmith_dataset.example.id
}

# Feedback formulas scoped to a project (session).
# Exactly one of dataset_id or session_id may be set.
data "langsmith_feedback_formulas" "by_project" {
  session_id = langsmith_project.example.id
}

output "dataset_formula_keys" {
  value = [for f in data.langsmith_feedback_formulas.by_dataset.formulas : f.feedback_key]
}
