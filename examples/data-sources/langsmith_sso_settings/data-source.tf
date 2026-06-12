# Reads the current organization's SSO settings. If the organization has
# multiple SSO configurations, specify `id` to select one.
data "langsmith_sso_settings" "example" {}
