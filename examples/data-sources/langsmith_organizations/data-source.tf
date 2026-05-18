data "langsmith_organizations" "all" {}

output "organization_ids" {
  value = [for o in data.langsmith_organizations.all.organizations : o.id]
}
