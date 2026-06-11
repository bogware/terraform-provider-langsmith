# Usage limits for the current workspace.
data "langsmith_usage_limits" "workspace" {}

# Usage limits across every workspace in the organization.
data "langsmith_usage_limits" "org" {
  org_scope = true
}

output "workspace_limits" {
  value = {
    for l in data.langsmith_usage_limits.workspace.limits : l.limit_type => l.limit_value
  }
}
