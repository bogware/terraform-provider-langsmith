# List the owners (collaborators) of a prompt repo in the current workspace.
data "langsmith_repo_owners" "example" {
  repo_handle = "my-prompt"
}

output "owner_emails" {
  value = [for o in data.langsmith_repo_owners.example.owners : o.email]
}
