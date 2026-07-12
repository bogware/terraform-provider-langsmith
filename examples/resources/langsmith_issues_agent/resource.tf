# Beta: attach the Issues Agent to a tracing project. Creating the resource
# enqueues the initial scan.
resource "langsmith_project" "chatbot" {
  name = "production-chatbot"
}

resource "langsmith_issues_agent" "chatbot" {
  session_id = langsmith_project.chatbot.id

  github_repo_url    = "https://github.com/acme/chatbot"
  github_base_branch = "main"
  github_repo_subdir = "services/chatbot"

  priorities        = ["P0", "P1"]
  cron_enabled      = true
  user_instructions = "Prioritize tool-call failures and hallucinated citations."

  # Seed the Agent Overview document. This is write-only: the API never returns
  # the content, so Terraform cannot detect drift on it and the value is not
  # populated on import. It is re-sent only when the configured content changes.
  overview = <<-EOT
    # Production chatbot

    Customer-facing support agent. Tools: `search_docs`, `create_ticket`.
    Escalate anything touching billing to a human.
  EOT

  # Monthly Engine LCU spend cap for this project (decimal string).
  session_lcu_spend_limit_monthly = "100"
}
