# List every sandbox registry configured in the workspace.
data "langsmith_sandbox_registries" "all" {}

# Or narrow the list to registries whose name contains a substring.
data "langsmith_sandbox_registries" "ghcr" {
  name_contains = "ghcr"
}

output "sandbox_registry_urls" {
  value = [for r in data.langsmith_sandbox_registries.all.registries : r.url]
}
