data "langsmith_dataset_versions" "example" {
  dataset_id = langsmith_dataset.example.id
}

# Pin a tag to the most recent version.
resource "langsmith_dataset_version_tag" "latest" {
  dataset_id = langsmith_dataset.example.id
  tag        = "reviewed"
  as_of      = data.langsmith_dataset_versions.example.versions[0].as_of
}
