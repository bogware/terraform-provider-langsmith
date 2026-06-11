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

  # Monthly Engine LCU spend cap for this project (decimal string).
  session_lcu_spend_limit_monthly = "100"
}
