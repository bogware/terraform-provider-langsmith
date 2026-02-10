resource "langsmith_access_policy" "example" {
  name   = "restrict-production"
  effect = "deny"
  condition_groups = jsonencode([
    {
      resource_type = "project"
      permission    = "write"
      conditions = [
        {
          field    = "name"
          operator = "equals"
          value    = "production"
        }
      ]
    }
  ])
}
