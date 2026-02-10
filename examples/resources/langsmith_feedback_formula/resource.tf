resource "langsmith_feedback_formula" "example" {
  feedback_key     = "composite_score"
  aggregation_type = "avg"
  formula_parts = jsonencode([
    {
      part_type = "weighted_key"
      weight    = 0.7
      key       = "correctness"
    },
    {
      part_type = "weighted_key"
      weight    = 0.3
      key       = "helpfulness"
    }
  ])
}
