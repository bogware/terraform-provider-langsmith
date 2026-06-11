data "langsmith_mcp_vendor" "openai" {
  vendor_slug = "openai"
}

resource "langsmith_mcp_vendor_settings" "openai" {
  vendor_slug     = data.langsmith_mcp_vendor.openai.vendor_slug
  organization_id = "org-abc123"
  project_id      = "proj-def456"
}
