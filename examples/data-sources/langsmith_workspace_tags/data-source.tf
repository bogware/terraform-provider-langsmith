# Discover the full tag taxonomy defined in the workspace.
data "langsmith_workspace_tags" "all" {}

# Only the tag keys that apply to projects.
data "langsmith_workspace_tags" "projects" {
  resource_type = "project"
}

# Every tag key name in the workspace.
output "tag_keys" {
  value = [for k in data.langsmith_workspace_tags.all.tag_keys : k.key]
}

# The values registered under the "Application" tag key, if it exists.
output "application_values" {
  value = flatten([
    for k in data.langsmith_workspace_tags.all.tag_keys :
    [for v in k.values : v.value] if k.key == "Application"
  ])
}
