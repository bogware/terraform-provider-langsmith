resource "langsmith_sso_settings" "example" {
  default_workspace_role_id = "00000000-0000-0000-0000-000000000000"
  metadata_url              = "https://login.example.com/saml/metadata"
}
