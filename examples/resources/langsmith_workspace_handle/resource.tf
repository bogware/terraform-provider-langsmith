# Assigns the public handle of the current workspace. Handles are globally
# unique and cannot be unset once assigned; destroying this resource only
# removes it from Terraform state.
resource "langsmith_workspace_handle" "this" {
  handle = "acme-research"
}
