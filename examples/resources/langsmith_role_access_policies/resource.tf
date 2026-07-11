# Attach a set of access policies to an organization role.
# This association is authoritative: any access policy not listed here is
# detached from the role on apply. Requires ABAC to be enabled on the org.
resource "langsmith_role_access_policies" "example" {
  role_id = "00000000-0000-0000-0000-000000000000"
  access_policy_ids = [
    "11111111-1111-1111-1111-111111111111",
    "22222222-2222-2222-2222-222222222222",
  ]
}
