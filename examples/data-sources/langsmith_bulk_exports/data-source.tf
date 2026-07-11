# List every bulk export job in the workspace.
data "langsmith_bulk_exports" "all" {}

output "bulk_export_ids" {
  value = [for e in data.langsmith_bulk_exports.all.bulk_exports : e.id]
}

# Only the exports that are still running.
output "running_bulk_exports" {
  value = [
    for e in data.langsmith_bulk_exports.all.bulk_exports : e.id
    if e.status == "Running"
  ]
}
