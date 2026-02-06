resource "langsmith_annotation_queue" "example" {
  name                   = "review-queue"
  description            = "Queue for human review of LLM outputs"
  num_reviewers_per_item = 2
}
