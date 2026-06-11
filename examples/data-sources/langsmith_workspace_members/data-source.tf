data "langsmith_workspace_members" "all" {}

output "workspace_member_emails" {
  value = [for m in data.langsmith_workspace_members.all.members : m.email]
}
