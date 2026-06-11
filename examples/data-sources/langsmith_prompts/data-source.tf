data "langsmith_prompts" "all" {}

data "langsmith_prompts" "active_private" {
  is_public   = false
  is_archived = "false"
}

output "prompt_handles" {
  value = [for p in data.langsmith_prompts.all.prompts : p.repo_handle]
}
