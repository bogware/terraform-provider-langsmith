resource "langsmith_hub_environment" "default" {
  environments = [
    { name = "staging" },
    { name = "production" },
  ]
}
