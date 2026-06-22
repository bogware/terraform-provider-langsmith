# Manage the longlived trace retention for a single workspace.
# This is a singleton resource: there is exactly one TTL configuration per
# workspace. It is distinct from the organization-level langsmith_ttl_settings.
resource "langsmith_workspace_ttl_settings" "example" {
  longlived_ttl_days = 400
}

# Target a specific workspace instead of the provider-level default.
resource "langsmith_workspace_ttl_settings" "other_workspace" {
  workspace_id       = "00000000-0000-0000-0000-000000000000"
  longlived_ttl_days = 90
}
