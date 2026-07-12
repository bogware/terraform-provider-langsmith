# List every LLM Gateway policy in the organization.
# Requires the LLM Gateway feature to be enabled on the organization;
# otherwise the API returns 403 "LLM Gateway not enabled for this organization".
data "langsmith_gateway_policies" "all" {}

# Filters are applied server-side. Pair subject_matcher_key with
# subject_matcher_value to find the policies governing one subject.
data "langsmith_gateway_policies" "spend_caps" {
  policy_type = "spend_cap"
}

data "langsmith_gateway_policies" "team_platform" {
  subject_matcher_key   = "team"
  subject_matcher_value = "platform"
}

# Report the spend caps that are currently over 80% consumed.
output "spend_caps_near_limit" {
  value = [
    for policy in data.langsmith_gateway_policies.spend_caps.gateway_policies : policy.name
    if policy.enabled
    && policy.current_spend_usd != null
    && policy.current_spend_usd > 0.8 * tonumber(jsondecode(policy.config).amount_usd)
  ]
}
