data "langsmith_session_agent_versions" "example" {
  session_id = langsmith_project.example.id
}

output "deployed_commits" {
  value = [for v in data.langsmith_session_agent_versions.example.versions : v.commit_sha]
}
