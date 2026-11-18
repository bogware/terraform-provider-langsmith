resource "langsmith_prompt" "example" {
  repo_handle     = "my-prompt"
  is_public       = false
  description     = "A reusable prompt template"
  restricted_mode = false
}

# A prompt whose content Terraform manages. Each change to manifest writes a
# new commit; commit_description labels it in the repo's history, and
# skip_webhooks keeps those commits from triggering prompt webhooks.
resource "langsmith_prompt" "versioned" {
  repo_handle = "support-triage"
  is_public   = false

  manifest = jsonencode({
    lc   = 1
    type = "constructor"
    id   = ["langchain", "prompts", "prompt", "PromptTemplate"]
    kwargs = {
      input_variables = ["ticket"]
      template        = "Classify this support ticket: {ticket}"
      template_format = "f-string"
    }
  })

  commit_description = "Tighten the triage instructions"
  skip_webhooks      = true
}
