## 0.5.4 (February 2026)

FEATURES:

* **New Resource:** `langsmith_project` - Manage tracing projects
* **New Resource:** `langsmith_dataset` - Manage evaluation datasets
* **New Resource:** `langsmith_example` - Manage dataset examples
* **New Resource:** `langsmith_annotation_queue` - Manage annotation queues for human review
* **New Resource:** `langsmith_service_account` - Manage service accounts
* **New Resource:** `langsmith_service_key` - Manage API service keys
* **New Resource:** `langsmith_prompt` - Manage prompts in the LangSmith Hub
* **New Resource:** `langsmith_run_rule` - Manage automation rules for runs
* **New Resource:** `langsmith_webhook` - Manage prompt webhooks
* **New Resource:** `langsmith_feedback_config` - Manage feedback score configurations
* **New Resource:** `langsmith_workspace` - Manage workspaces
* **New Resource:** `langsmith_tag_key` - Manage tag keys
* **New Resource:** `langsmith_tag_value` - Manage tag values
* **New Resource:** `langsmith_bulk_export_destination` - Manage bulk export S3 destinations
* **New Resource:** `langsmith_bulk_export` - Manage bulk export jobs
* **New Resource:** `langsmith_model_price_map` - Manage model pricing configuration
* **New Resource:** `langsmith_usage_limit` - Manage usage limits
* **New Resource:** `langsmith_playground_settings` - Manage playground settings
* **New Data Source:** `langsmith_project` - Look up a project by name or ID
* **New Data Source:** `langsmith_dataset` - Look up a dataset by name or ID
* **New Data Source:** `langsmith_workspace` - Look up a workspace by name or ID
* **New Data Source:** `langsmith_info` - Retrieve LangSmith server information
* **New Data Source:** `langsmith_organization` - Retrieve current organization information

ENHANCEMENTS:

* Provider supports `tenant_id` for org-scoped API key authentication
* Immutable fields marked with `RequiresReplace` plan modifiers across all resources
* Proper null handling in all response-to-state mappers to prevent drift
* Feedback config resource gracefully handles external deletion via `RemoveResource`
* Run rule defaults for `add_to_dataset_prefer_correction` and `num_few_shot_examples` prevent perpetual diffs
