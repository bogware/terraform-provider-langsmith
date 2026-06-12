# List all datasets in the workspace.
data "langsmith_datasets" "all" {}

# List only chat datasets whose name contains a substring.
data "langsmith_datasets" "chat" {
  name_contains = "eval"
  data_type     = "chat"
}

output "dataset_names" {
  value = [for d in data.langsmith_datasets.all.datasets : d.name]
}
