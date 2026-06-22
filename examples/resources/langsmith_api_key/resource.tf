# A durable workspace-scoped API key. The full key is only returned at
# creation time, so capture it from state immediately if you need it.
resource "langsmith_api_key" "ci" {
  description = "CI pipeline key"
  expires_at  = "2030-01-01T00:00:00Z"
}

output "ci_api_key" {
  value     = langsmith_api_key.ci.key
  sensitive = true
}
