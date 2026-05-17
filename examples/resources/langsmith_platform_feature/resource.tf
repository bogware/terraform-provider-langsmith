# Copyright (c) Bogware, Inc. 2025
# SPDX-License-Identifier: MPL-2.0

# Platform feature model gates are workspace/org scoped. Set LANGSMITH_ORGANIZATION_ID
# on the provider when your API key requires X-Organization-Id. Replace feature and
# model names with values returned by data.langsmith_platform_features.

resource "langsmith_platform_feature" "example" {
  feature         = "playground"
  default_model   = "claude-3-5-sonnet-20240620"
  disabled_models = ["gpt-4", "gpt-4o-mini"]
}
