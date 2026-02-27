provider "langsmith" {
  api_key = var.langsmith_api_key
}

data "langsmith_user" "example" {
  email = "user@example.com"
}

output "user_id" {
  value = data.langsmith_user.example.id
}

output "user_display_name" {
  value = data.langsmith_user.example.display_name
}

