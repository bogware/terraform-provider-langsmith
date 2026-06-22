resource "langsmith_comparative_experiment" "example" {
  reference_dataset_id = "00000000-0000-0000-0000-000000000000"
  experiment_ids = [
    "11111111-1111-1111-1111-111111111111",
    "22222222-2222-2222-2222-222222222222",
  ]
  name        = "baseline-vs-candidate"
  description = "Compares the baseline and candidate experiments on the shared dataset."
  extra       = jsonencode({ owner = "evals-team" })
}
