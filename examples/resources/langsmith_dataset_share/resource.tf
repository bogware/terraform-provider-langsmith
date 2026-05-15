resource "langsmith_dataset_share" "golden" {
  dataset_id     = langsmith_dataset.golden.id
  share_projects = true
}

output "share_token" {
  value = langsmith_dataset_share.golden.share_token
}
