data "langsmith_organization" "current" {}

output "org_name" {
  value = data.langsmith_organization.current.display_name
}

output "org_tier" {
  value = data.langsmith_organization.current.tier
}
