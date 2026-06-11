# Manages settings on the organization the API key belongs to. Only the
# attributes you set are sent to the API; everything else is left untouched.
# Destroying this resource removes it from Terraform state only.
resource "langsmith_organization_settings" "this" {
  display_name            = "Acme Research"
  public_sharing_disabled = true
  pat_creation_disabled   = false
  security_contact        = "security@example.com"
  max_api_key_expiry_days = 90
}
