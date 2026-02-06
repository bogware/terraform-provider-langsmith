resource "langsmith_dataset" "example" {
  name        = "my-dataset"
  description = "A dataset for evaluation"
  data_type   = "kv"
}
