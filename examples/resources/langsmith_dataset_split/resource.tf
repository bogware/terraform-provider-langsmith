resource "langsmith_dataset_split" "train" {
  dataset_id = langsmith_dataset.golden.id
  name       = "train"
  example_ids = [
    langsmith_example.first.id,
    langsmith_example.second.id,
  ]
}
