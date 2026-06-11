data "langsmith_playground_settings" "all" {}

output "playground_setting_names" {
  value = [for s in data.langsmith_playground_settings.all.settings : s.name]
}
