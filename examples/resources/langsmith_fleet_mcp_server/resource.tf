resource "langsmith_fleet_mcp_server" "example" {
  name      = "Example MCP"
  url       = "https://mcp.example.com/sse"
  auth_type = "headers"
  headers   = jsonencode([{ Authorization = "Bearer replace-me" }])
}
