resource "langsmith_usage_limit" "example" {
  limit_type  = "traces"
  limit_value = 100000
}
