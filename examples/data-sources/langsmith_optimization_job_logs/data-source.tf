data "langsmith_optimization_job_logs" "example" {
  owner  = langsmith_optimization_job.example.owner
  repo   = langsmith_optimization_job.example.repo
  job_id = langsmith_optimization_job.example.id
}

output "optimization_errors" {
  value = [
    for l in data.langsmith_optimization_job_logs.example.logs : l.message
    if l.log_type == "error"
  ]
}
