resource "langsmith_feedback_config" "example" {
  feedback_key  = "correctness"
  feedback_type = "continuous"
  min           = 0
  max           = 1
}
