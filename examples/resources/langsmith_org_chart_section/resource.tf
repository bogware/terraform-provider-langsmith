# Organization chart sections require X-Organization-Id. Set provider
# organization_id or LANGSMITH_ORGANIZATION_ID (for example from
# data.langsmith_organization in a separate workspace / provider alias).

resource "langsmith_org_chart_section" "example" {
  title       = "Org KPIs"
  description = "Organization-wide dashboards"
}
