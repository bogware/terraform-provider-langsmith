resource "langsmith_prompt_tag" "example" {
  repo_handle = langsmith_prompt.example.repo_handle
  tag_name    = "production"
  commit_hash = "abc123"
}
