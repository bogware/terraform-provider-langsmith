resource "langsmith_service_key" "example" {
  description = "API key for CI/CD pipeline"
  read_only   = false
}
