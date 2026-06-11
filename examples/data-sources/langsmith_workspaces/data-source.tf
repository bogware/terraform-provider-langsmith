# List every workspace in the organization.
data "langsmith_workspaces" "all" {}

# Create the same tracing project in every workspace by combining the
# workspace list with resource-level workspace_id.
resource "langsmith_project" "tracing" {
  for_each = { for w in data.langsmith_workspaces.all.workspaces : w.id => w }

  name         = "tracing"
  workspace_id = each.key
}

output "workspace_names" {
  value = [for w in data.langsmith_workspaces.all.workspaces : w.display_name]
}
