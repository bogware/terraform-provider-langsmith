resource "langsmith_model_price_map" "example" {
  name            = "gpt-4o"
  match_pattern   = "gpt-4o.*"
  prompt_cost     = 0.0000025
  completion_cost = 0.00001
  model_provider  = "openai"
}
