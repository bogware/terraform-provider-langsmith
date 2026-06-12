resource "langsmith_feature_model_config" "playground" {
  feature       = "playground"
  default_model = "gpt-4o-mini"

  disabled_models = [
    "gpt-3.5-turbo",
    "gpt-4",
  ]
}
