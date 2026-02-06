resource "langsmith_tag_value" "example" {
  tag_key_id  = langsmith_tag_key.example.id
  value       = "production"
  description = "Production environment"
}
