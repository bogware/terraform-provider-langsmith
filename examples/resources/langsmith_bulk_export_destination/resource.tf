resource "langsmith_bulk_export_destination" "example" {
  display_name      = "my-s3-export"
  destination_type  = "s3"
  bucket_name       = "my-langsmith-exports"
  prefix            = "traces/"
  region            = "us-east-1"
  access_key_id     = var.aws_access_key_id
  secret_access_key = var.aws_secret_access_key
}
