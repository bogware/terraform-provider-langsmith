# List every bulk export destination in the workspace.
# Credential values (access key ID, secret access key, session token) are never
# returned by the LangSmith API -- only the configured credential key names are.
data "langsmith_bulk_export_destinations" "all" {}

output "destination_buckets" {
  value = {
    for d in data.langsmith_bulk_export_destinations.all.destinations :
    d.display_name => d.bucket_name
  }
}

# Look up a destination by display name and reuse it for a bulk export.
locals {
  archive_destination_id = one([
    for d in data.langsmith_bulk_export_destinations.all.destinations : d.id
    if d.display_name == "archive"
  ])
}
