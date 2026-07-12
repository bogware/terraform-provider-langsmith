# List every example in a dataset.
data "langsmith_examples" "all" {
  dataset_id = langsmith_dataset.example.id
}

# Only the examples in the "test" split, as of the "latest" dataset version.
data "langsmith_examples" "test_split" {
  dataset_id = langsmith_dataset.example.id
  splits     = ["test"]
  as_of      = "latest"
}

# Filter by metadata and cap the number of results returned.
data "langsmith_examples" "from_prod" {
  dataset_id = langsmith_dataset.example.id
  metadata   = jsonencode({ source = "prod" })
  limit      = 25
  offset     = 0
}

output "example_count" {
  value = length(data.langsmith_examples.all.examples)
}

output "first_example_inputs" {
  value = try(data.langsmith_examples.all.examples[0].inputs, null)
}
