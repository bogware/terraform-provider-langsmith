resource "langsmith_tag_key" "environment" {
  key         = "environment"
  description = "Deployment environment tag"
}

resource "langsmith_tag_value" "production" {
  tag_key_id  = langsmith_tag_key.environment.id
  value       = "production"
  description = "Production environment"
}

resource "langsmith_project" "example" {
  name        = "my-project"
  description = "A project for tracing LLM runs"

  # Associate tag values with the project (see langsmith_tag_value).
  tag_value_ids = [langsmith_tag_value.production.id]
}
