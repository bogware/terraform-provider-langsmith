# Audit everything currently exposed through a public share link.
data "langsmith_shared_tokens" "example" {}

output "shared_entity_count" {
  value = data.langsmith_shared_tokens.example.entity_count
}

output "shared_entities" {
  value = jsondecode(data.langsmith_shared_tokens.example.entities)
}
