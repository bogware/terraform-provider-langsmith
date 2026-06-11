# List all permissions available for authoring custom roles.
data "langsmith_permissions" "all" {}

output "workspace_scoped_permissions" {
  value = [
    for p in data.langsmith_permissions.all.permissions : p.name
    if p.access_scope == "workspace"
  ]
}
