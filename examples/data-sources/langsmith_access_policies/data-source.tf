# List every ABAC access policy in the organization.
# Requires ABAC to be enabled on the organization (enterprise tier);
# otherwise the API returns 403 "ABAC is not enabled for this organization".
data "langsmith_access_policies" "all" {}

resource "langsmith_org_role" "reader" {
  display_name = "custom-reader"
  description  = "Read-only access"
  permissions  = jsonencode(["read_runs", "read_datasets"])
}

# Discover policy IDs by name instead of hard-coding UUIDs, then attach them
# to a role.
resource "langsmith_role_access_policies" "reader" {
  role_id = langsmith_org_role.reader.id
  access_policy_ids = [
    for policy in data.langsmith_access_policies.all.access_policies :
    policy.id if startswith(policy.name, "reader-") && policy.effect == "allow"
  ]
}

output "access_policy_names" {
  value = [for policy in data.langsmith_access_policies.all.access_policies : policy.name]
}
