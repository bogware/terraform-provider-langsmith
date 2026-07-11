# List all platform tools in the workspace.
data "langsmith_tools" "all" {}

output "tool_handles" {
  value = [for t in data.langsmith_tools.all.tools : t.handle]
}
