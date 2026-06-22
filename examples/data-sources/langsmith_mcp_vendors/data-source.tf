data "langsmith_mcp_vendors" "all" {}

output "mcp_vendor_ids" {
  value = [for v in data.langsmith_mcp_vendors.all.vendors : v.vendor_id]
}
