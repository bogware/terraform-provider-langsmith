resource "langsmith_prompt" "example" {
  repo_handle = "my-prompt"
  is_public   = false
  description = "A reusable prompt template"
}
