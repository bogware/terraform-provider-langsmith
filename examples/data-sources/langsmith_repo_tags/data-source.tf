# The full prompt-repo tag catalog, with a usage count per tag.
data "langsmith_repo_tags" "all" {}

# Narrow the catalog with a search string.
data "langsmith_repo_tags" "chat" {
  query = "Chat"
}

# Tag name -> number of prompt repos carrying it.
output "tag_counts" {
  value = { for t in data.langsmith_repo_tags.all.tags : t.tag => t.count }
}

# Only the tags that are in wide circulation.
output "common_tags" {
  value = [for t in data.langsmith_repo_tags.all.tags : t.tag if t.count >= 100]
}
