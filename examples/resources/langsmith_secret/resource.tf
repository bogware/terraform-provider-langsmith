resource "langsmith_secret" "example" {
  key   = "OPENAI_API_KEY"
  value = var.openai_api_key
}
