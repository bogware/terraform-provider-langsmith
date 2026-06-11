# List all projects in the workspace.
data "langsmith_projects" "all" {}

# List projects whose name contains a substring, in a specific workspace.
data "langsmith_projects" "prod" {
  name_contains = "prod"
  workspace_id  = "00000000-0000-0000-0000-000000000000"
}

output "project_names" {
  value = [for p in data.langsmith_projects.all.projects : p.name]
}
