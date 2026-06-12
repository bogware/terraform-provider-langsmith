# Lists the key names of workspace secrets. Secret values are never
# returned by the LangSmith API -- only the names are available.
data "langsmith_secret_names" "all" {}

output "secret_names" {
  value = data.langsmith_secret_names.all.names
}
