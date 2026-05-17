resource "langsmith_api_key" "automation" {
  description = "Example tenant API key managed by Terraform"
  read_only   = false
}
