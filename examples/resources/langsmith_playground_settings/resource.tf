resource "langsmith_playground_settings" "example" {
  name     = "default-settings"
  settings = jsonencode({ temperature = 0.7, max_tokens = 1000 })
}
