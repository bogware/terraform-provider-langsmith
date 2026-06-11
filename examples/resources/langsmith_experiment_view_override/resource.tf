resource "langsmith_dataset" "qa" {
  name      = "qa-dataset"
  data_type = "kv"
}

resource "langsmith_experiment_view_override" "qa" {
  dataset_id = langsmith_dataset.qa.id

  column_overrides = [
    {
      column         = "outputs.accuracy"
      precision      = 3
      color_gradient = jsonencode([[0, "#ff0000"], [0.5, "#ffff00"], [1, "#00ff00"]])
    },
    {
      column = "inputs.internal_notes"
      hide   = true
    },
  ]
}
