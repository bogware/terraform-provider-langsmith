resource "langsmith_tagging" "example" {
  tag_value_id  = langsmith_tag_value.example.id
  resource_type = "project"
  resource_id   = langsmith_project.example.id
}
