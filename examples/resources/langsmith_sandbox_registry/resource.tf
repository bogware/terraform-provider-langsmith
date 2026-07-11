variable "registry_password" {
  description = "Password or access token for the private container registry."
  type        = string
  sensitive   = true
}

# A private container image registry that LangSmith sandboxes pull images from.
#
# `username` and `password` are write-only: the LangSmith API accepts them but
# never returns them, so Terraform cannot detect drift on them and cannot
# recover them on import.
resource "langsmith_sandbox_registry" "example" {
  name     = "my-private-registry"
  url      = "ghcr.io"
  username = "my-registry-user"
  password = var.registry_password
}
