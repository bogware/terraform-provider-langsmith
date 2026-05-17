# Copyright (c) Bogware, Inc. 2025
# SPDX-License-Identifier: MPL-2.0

# Code evaluator (OpenAPI `evaluators.EvaluatorType` = `code`)
resource "langsmith_evaluator" "code_example" {
  name = "my-code-evaluator"
  type = "code"
  code_evaluator = jsonencode({
    code     = <<-EOT
      def score(run, example=None, **kwargs):
          return {"key": "pass", "score": True}
    EOT
    language = "python"
  })
}

# LLM evaluator backed by a Hub prompt (OpenAPI `type` = `llm`)
resource "langsmith_evaluator" "llm_example" {
  name = "my-llm-evaluator"
  type = "llm"
  llm_evaluator = jsonencode({
    prompt_repo_handle = "my-org/my-prompt"
    commit_hash_or_tag = "latest"
    variable_mapping   = { input = "inputs.question" }
  })
}
