resource "langsmith_chart_section_clone" "example" {
  source_section_id = langsmith_chart_section.production.id

  # Optional follow-up edits applied after the clone.
  title       = "Staging Health"
  description = "Cloned from Production for staging environment dashboards."
}
