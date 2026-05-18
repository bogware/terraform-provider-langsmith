data "langsmith_organization_pending_invites" "pending" {}

output "pending_org_ids" {
  value = [for o in data.langsmith_organization_pending_invites.pending.pending_invites : o.id]
}
