resource "langsmith_org_member" "example" {
  email   = "user@example.com"
  role_id = langsmith_org_role.example.id
}
