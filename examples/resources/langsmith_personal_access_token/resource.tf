resource "langsmith_personal_access_token" "ci" {
  description          = "CI automation token"
  expires_at           = "2027-01-01T00:00:00Z"
  default_workspace_id = langsmith_workspace.production.id
}

output "pat" {
  value     = langsmith_personal_access_token.ci.key
  sensitive = true
}
