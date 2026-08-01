resource "langsmith_oauth_client" "example" {
  client_name = "Internal Reporting Tool"
  client_type = "confidential"
  client_uri  = "https://reporting.example.com"

  redirect_uris = ["https://reporting.example.com/oauth/callback"]
  grant_types   = ["authorization_code", "refresh_token"]
}

# The secret is only returned at registration -- capture it in the same apply.
output "oauth_client_secret" {
  value     = langsmith_oauth_client.example.client_secret
  sensitive = true
}
