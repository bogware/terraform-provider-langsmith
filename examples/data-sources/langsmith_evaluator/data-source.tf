# Copyright (c) Bogware, Inc. 2025
# SPDX-License-Identifier: MPL-2.0

data "langsmith_evaluator" "existing" {
  evaluator_id = "00000000-0000-4000-8000-000000000001"
}

output "evaluator_name" {
  value = data.langsmith_evaluator.existing.name
}
