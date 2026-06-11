data "langsmith_org_members" "all" {}

output "org_member_emails" {
  value = [for m in data.langsmith_org_members.all.members : m.email]
}

output "pending_invitations" {
  value = [for p in data.langsmith_org_members.all.pending : p.email]
}
