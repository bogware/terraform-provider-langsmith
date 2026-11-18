# A composite score: the weighted average of two feedback keys, computed for
# every run in the project once both keys have been scored.
#
# This replaces the retired langsmith_feedback_formula and takes the same
# inputs. Changing any of them replaces the composite evaluator.
resource "langsmith_composite_evaluator" "quality" {
  feedback_key     = "quality"
  aggregation_type = "avg"
  session_id       = langsmith_project.example.id

  formula_parts = jsonencode([
    { part_type = "weighted_key", weight = 1.0, key = "correctness" },
    { part_type = "weighted_key", weight = 0.5, key = "helpfulness" },
  ])
}
