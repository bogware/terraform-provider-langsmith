# DEPRECATED: LangSmith has retired the feedback formula endpoints (every call
# returns 410 Gone), so this data source no longer works and will be removed in the
# next major version. Composite scores are now created as a code evaluator plus
# a run rule. The example below is kept for reference only.
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
