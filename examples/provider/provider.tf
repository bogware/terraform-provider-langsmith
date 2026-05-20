provider "langsmith" {
  api_key      = var.langsmith_api_key
  api_url      = "https://api.smith.langchain.com"
  workspace_id = var.LANGSMITH_WORKSPACE_ID # Required for org-scoped API keys
}
