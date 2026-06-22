data "langsmith_bulk_export_destination" "example" {
  id = "00000000-0000-0000-0000-000000000000"
}

output "destination_bucket" {
  value = data.langsmith_bulk_export_destination.example.bucket_name
}
