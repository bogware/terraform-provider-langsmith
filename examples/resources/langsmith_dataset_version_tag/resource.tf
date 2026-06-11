resource "langsmith_dataset" "golden" {
  name      = "golden-questions"
  data_type = "kv"
}

# Pin the `prod` tag to the most recent dataset version. Use the exact
# timestamp of an existing dataset version instead of "latest" to pin a
# specific known-good snapshot.
resource "langsmith_dataset_version_tag" "prod" {
  dataset_id = langsmith_dataset.golden.id
  tag        = "prod"
  as_of      = "latest"
}

output "prod_version" {
  value = langsmith_dataset_version_tag.prod.version_as_of
}
