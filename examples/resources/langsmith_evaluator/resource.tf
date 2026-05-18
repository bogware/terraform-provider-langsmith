resource "langsmith_evaluator" "llm_judge" {
  name = "answer-correctness"
  type = "llm"

  llm_evaluator = {
    prompt_repo_handle = "my-team/correctness-judge"
    commit_hash_or_tag = "production"
    variable_mapping = jsonencode({
      question = "input.question"
      answer   = "output.answer"
    })
  }
}

resource "langsmith_evaluator" "code_check" {
  name = "json-shape"
  type = "code"

  code_evaluator = {
    # The entry-point must be named `perform_eval(run, example)`.
    code     = <<-EOT
      def perform_eval(run, example):
          try:
              import json
              json.loads(run.outputs["raw"])
              return {"score": 1}
          except Exception:
              return {"score": 0}
    EOT
    language = "python"
  }
}
