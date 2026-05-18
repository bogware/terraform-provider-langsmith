data "langsmith_organization_permissions" "catalog" {}

output "workspace_scoped_permissions" {
  value = [for p in data.langsmith_organization_permissions.catalog.permissions : p.name if p.access_scope == "workspace"]
}
