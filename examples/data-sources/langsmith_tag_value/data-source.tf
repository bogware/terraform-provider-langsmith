data "langsmith_tag_key" "environment" {
  key = "environment"
}

# Look up by value name
data "langsmith_tag_value" "production" {
  tag_key_id = data.langsmith_tag_key.environment.id
  value      = "production"
}

# Or look up by ID
data "langsmith_tag_value" "by_id" {
  tag_key_id = data.langsmith_tag_key.environment.id
  id         = "00000000-0000-0000-0000-000000000000"
}
