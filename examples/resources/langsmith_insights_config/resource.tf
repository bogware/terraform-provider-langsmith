resource "langsmith_insights_config" "topics" {
  session_id    = langsmith_project.production.id
  name          = "Daily topic clusters"
  description   = "Group last-24h runs by user intent"
  schedule_cron = "0 3 * * *"

  # See LangSmith docs: CreateRunClusteringJobRequest.
  config = jsonencode({
    last_n_hours = 24
    model        = "openai"
    hierarchy    = [10, 50]
  })
}
