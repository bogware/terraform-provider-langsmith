data "langsmith_org_usage" "january" {
  starting_on   = "2025-01-01T00:00:00Z"
  ending_before = "2025-02-01T00:00:00Z"
}

output "usage_by_metric" {
  value = {
    for entry in data.langsmith_org_usage.january.usage :
    entry.billable_metric_name => entry.value...
  }
}
