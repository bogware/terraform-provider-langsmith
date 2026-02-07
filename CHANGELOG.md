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
* **New Resource:** `langsmith_secret` - Manage workspace secrets (key/value store)
* **New Resource:** `langsmith_ttl_settings` - Manage trace retention (TTL) settings
* **New Resource:** `langsmith_alert_rule` - Manage alert rules for project monitoring
* **New Resource:** `langsmith_org_role` - Manage organization roles (RBAC)
* **New Resource:** `langsmith_sso_settings` - Manage SSO/SAML settings
* **New Resource:** `langsmith_workspace_member` - Manage workspace members
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
* Project resource now supports `trace_tier` for controlling trace retention
* Dataset resource now exposes `transformations`, `metadata`, and computed stats (`example_count`, `session_count`, `modified_at`)
* Run rule resource now supports evaluators, code evaluators, alerts, webhooks, dataset_id, group_by, and all boolean flags
* Annotation queue resource now supports `rubric_items`, `metadata`, and computed `queue_type`/`source_rule_id`/`run_rule_id`
* Prompt resource now supports `is_archived` and computed stats (num_commits, num_likes, etc.)
* Bulk export resource now supports `format_version`, `export_fields`, and computed `finished_at`
* Playground settings resource now supports `options` and `settings_type`
* Service key resource now supports `expires_at`, `default_workspace_id`, and `role_id`
* Model price map resource now supports `prompt_cost_details` and `completion_cost_details`
* Workspace resource now exposes computed `organization_id` and `is_personal`
