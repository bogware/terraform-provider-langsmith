# DEPRECATED: LangSmith has retired the feedback formula endpoints (every call
# returns 410 Gone), so this resource no longer works and will be removed in the
# next major version. Composite scores are now created as a code evaluator plus
# a run rule. The example below is kept for reference only.
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
