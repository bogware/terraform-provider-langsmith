resource "langsmith_org_role" "example" {
  display_name = "custom-viewer"
  description  = "A custom role with read-only access"
  permissions  = jsonencode(["read_runs", "read_datasets"])
}
