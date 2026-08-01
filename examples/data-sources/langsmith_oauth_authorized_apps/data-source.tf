data "langsmith_oauth_authorized_apps" "example" {}

output "authorized_app_names" {
  value = [for a in data.langsmith_oauth_authorized_apps.example.apps : a.client_name]
}
