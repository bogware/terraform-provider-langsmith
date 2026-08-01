resource "langsmith_optimization_job" "example" {
  owner     = "-"
  repo      = langsmith_prompt.example.repo_handle
  algorithm = "promptim"

  config = jsonencode({
    dataset_id = langsmith_dataset.example.id
    max_steps  = 10
  })
}
