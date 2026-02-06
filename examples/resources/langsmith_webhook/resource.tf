resource "langsmith_webhook" "example" {
  url      = "https://example.com/webhook"
  triggers = ["on_commit"]
}
