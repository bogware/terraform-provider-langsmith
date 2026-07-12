resource "langsmith_org_role" "example" {
  display_name = "custom-viewer"
  description  = "A custom role with read-only access"
  permissions  = jsonencode(["read_runs", "read_datasets"])
}

# A restricted role can only be handed out by organization admins. The flag is
# updatable in place — flipping it does not replace the role.
resource "langsmith_org_role" "restricted_auditor" {
  display_name  = "restricted-auditor"
  description   = "Break-glass auditing role, admin-assignable only"
  permissions   = jsonencode(["read_runs", "read_datasets", "read_annotation_queues"])
  is_restricted = true
}
