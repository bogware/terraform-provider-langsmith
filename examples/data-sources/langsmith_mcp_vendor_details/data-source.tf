# Each section is fetched only when asked for.
data "langsmith_mcp_vendor_details" "example" {
  vendor_slug   = "slack"
  include_tools = true
}

output "vendor_tools" {
  value = jsondecode(data.langsmith_mcp_vendor_details.example.tools)
}
