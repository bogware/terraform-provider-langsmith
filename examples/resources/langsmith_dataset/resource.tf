resource "langsmith_dataset" "example" {
  name        = "my-dataset"
  description = "A dataset for evaluation"
  data_type   = "kv"

  # Optionally pin an experiment as the comparison baseline for this dataset.
  # baseline_experiment_id = "00000000-0000-0000-0000-000000000000"
}
