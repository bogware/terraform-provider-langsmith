data "langsmith_prompt_commit" "example" {
  repo_handle = "my-prompt"
  ref         = "latest"
}
