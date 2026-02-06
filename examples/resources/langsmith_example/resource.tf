resource "langsmith_example" "example" {
  dataset_id = langsmith_dataset.example.id
  inputs     = jsonencode({ question = "What is LangSmith?" })
  outputs    = jsonencode({ answer = "LangSmith is an LLM observability platform." })
}
