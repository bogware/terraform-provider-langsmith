resource "langsmith_annotation_queue" "example" {
  name = "example-review-queue"
}

# identity_id is typically a workspace member id from
# GET /api/v1/workspaces/current/members (the member's "id" field).
resource "langsmith_annotation_queue_reviewer" "example" {
  queue_id    = langsmith_annotation_queue.example.id
  identity_id = "00000000-0000-4000-8000-000000000000"
}
