resource "langsmith_annotation_queue_reviewer" "alice" {
  queue_id    = langsmith_annotation_queue.review.id
  identity_id = data.langsmith_user.alice.id
}
