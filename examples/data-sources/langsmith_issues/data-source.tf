# Beta: list all issues in the workspace
data "langsmith_issues" "all" {}

# Beta: filter issues to a single tracing project
data "langsmith_issues" "project" {
  session_id = "00000000-0000-0000-0000-000000000000"
}

output "issue_count" {
  value = length(data.langsmith_issues.all.issues)
}

output "issues" {
  value = [for i in data.langsmith_issues.all.issues : jsondecode(i)]
}
