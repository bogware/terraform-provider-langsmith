resource "langsmith_repo_owner" "bob" {
  owner = "my-workspace"
  repo  = "shared-prompts"
  email = "bob@example.com"
}
