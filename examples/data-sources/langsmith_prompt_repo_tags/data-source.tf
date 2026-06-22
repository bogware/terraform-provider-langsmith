data "langsmith_prompt_repo_tags" "example" {
  repo_handle = "my-prompt"
}

output "tag_names" {
  value = [for t in data.langsmith_prompt_repo_tags.example.tags : t.tag_name]
}
