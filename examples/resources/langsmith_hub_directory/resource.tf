resource "langsmith_hub_directory" "example" {
  owner = "-"
  repo  = "shared-prompts"

  # The whole directory: a path omitted here is removed by the commit.
  files = jsonencode({
    "README.md"         = "# Shared prompts\n"
    "prompts/greet.txt" = "You are a helpful assistant.\n"
  })
}
