resource "langsmith_service_key" "example" {
  description = "API key for CI/CD pipeline"
  read_only   = false

  # role_id and org_role_id can be changed in place (without rotating the key).
  # Updating either requires organization admin permissions.
  role_id = "00000000-0000-0000-0000-000000000000"
}
