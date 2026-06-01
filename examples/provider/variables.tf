variable "langsmith_api_key" {
  type        = string
  sensitive   = true
  description = "LangSmith API key. Can also be supplied via the LANGSMITH_API_KEY environment variable instead of this variable."
}

variable "langsmith_workspace_id" {
  type        = string
  default     = null
  description = "LangSmith workspace ID (required for org-scoped API keys). Can also be supplied via the LANGSMITH_WORKSPACE_ID environment variable."
}

