# Copyright (c) Bogware, Inc. 2025
# SPDX-License-Identifier: MPL-2.0

data "langsmith_platform_features" "example" {}

output "feature_keys" {
  value = [for f in data.langsmith_platform_features.example.features : f.feature]
}
