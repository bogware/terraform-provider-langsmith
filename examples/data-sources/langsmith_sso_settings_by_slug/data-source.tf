# Example: discover which organizations expose SSO for a login slug.
# The API returns a list of SSOProviderSlim objects (see LangSmith OpenAPI).

data "langsmith_sso_settings_by_slug" "example" {
  sso_login_slug = "my-org-slug"
}

output "sso_providers_for_slug" {
  value = data.langsmith_sso_settings_by_slug.example.providers
}
