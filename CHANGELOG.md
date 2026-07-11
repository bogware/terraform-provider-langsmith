## 0.10.1 (Unreleased)

BUG FIXES:

* Resources with multi-step creates no longer orphan the remote object when a follow-up step fails (#61). Previously, e.g., `langsmith_prompt` could create the prompt repo and then fail on the initial manifest commit (such as a 400 for an unsupported manifest type) without recording anything in state, so the next apply hit `409 already exists`. Affected creates now persist partial state on follow-up failures, so Terraform tracks the resource as tainted and replaces it cleanly on the next apply. Hardened: `langsmith_prompt`, `langsmith_chart_section_clone`, `langsmith_issues_agent`, `langsmith_feature_model_config`, `langsmith_access_policy`, `langsmith_feedback_config`, `langsmith_hub_environment`, `langsmith_org_member`, `langsmith_workspace_member`.

## 0.10.0 (June 2026)

BUG FIXES:

* Many resources could fail apply with "Provider returned invalid result object after apply: ... workspace_id ... unknown" or leave `workspace_id` null in state. All resources and data sources now resolve `workspace_id` to a known value after every operation: the API-returned value when available (decoding both the `workspace_id` and legacy `tenant_id` response keys — several endpoints only return the latter), falling back to the workspace the provider is configured against. Affected: `langsmith_project`, `langsmith_dataset`, `langsmith_prompt`, `langsmith_annotation_queue`, `langsmith_tool`, `langsmith_alert_rule`, `langsmith_chart`, `langsmith_chart_section`, `langsmith_chart_section_clone`, `langsmith_org_chart`, `langsmith_org_chart_section`, `langsmith_example`, `langsmith_secret`, `langsmith_tagging`, `langsmith_tag_key`, `langsmith_tag_value`, `langsmith_dataset_share`, `langsmith_dataset_split`, `langsmith_insights_config`, `langsmith_feedback_formula`, and the corresponding data sources.
* `langsmith_feedback_config`: a server-returned JSON `null` for `categories` was stored as the literal string `"null"`, causing "inconsistent result after apply" errors.

FEATURES:

* **New Resource:** `langsmith_experiment_view_override` - Per-dataset experiment view column configuration
* **New Resource:** `langsmith_feature_model_config` - Default/disabled model configuration per platform feature
* **New Resource:** `langsmith_mcp_vendor_settings` - MCP vendor settings (org/project bindings per vendor)
* **New Resource:** `langsmith_agent_builder_integrations` - Workspace Agent Builder integrations settings (singleton)
* **New Resource:** `langsmith_issues_agent` - **Beta:** per-project issues agent configuration
* **New Resource:** `langsmith_data_plane` - Self-hosted data planes (create-only; deprovisioning requires support)
* **New Resource:** `langsmith_dataset_version_tag` - Named tags on dataset versions (e.g. pin `prod` to a snapshot; `as_of` accepts a version timestamp or `latest`)
* **New Resource:** `langsmith_run_share` - Public share state for a run (mirror of `langsmith_dataset_share`)
* **New Resource:** `langsmith_workspace_handle` - Workspace hub handle (set-only; handles cannot be unset)
* **New Resource:** `langsmith_organization_settings` - Current-organization settings (display name, login methods, security toggles)
* **New Data Source:** `langsmith_workspaces` - List all workspaces; combine with `for_each` and resource-level `workspace_id` for dynamic multi-workspace management (#21)
* **New Data Source:** `langsmith_permissions` - Org permission catalog for authoring `langsmith_org_role`
* **New Data Source:** `langsmith_projects`, `langsmith_datasets`, `langsmith_prompts`, `langsmith_evaluators`, `langsmith_run_rules` - Filterable plural listings with pagination
* **New Data Source:** `langsmith_playground_settings`, `langsmith_usage_limits`, `langsmith_hub_environments`, `langsmith_secret_names` - Workspace-level listings (secret values are never returned)
* **New Data Source:** `langsmith_workspace_members`, `langsmith_org_members` - Member and pending-invite listings
* **New Data Source:** `langsmith_example`, `langsmith_feedback_config`, `langsmith_filter_view`, `langsmith_tag_value`, `langsmith_bulk_export`, `langsmith_sso_settings` - Singular lookups for existing resource types
* **New Data Source:** `langsmith_workspace_stats`, `langsmith_org_usage`, `langsmith_evaluator_spend` - Usage and statistics reads
* **New Data Source:** `langsmith_issues` - **Beta:** list detected issues, optionally per project

## 0.9.0 / 0.9.1 (May–June 2026)

BREAKING CHANGES:

* `langsmith_prompt` resource: removed the `num_likes`, `num_views`, `num_downloads`, `num_commits`, and `last_commit_hash` attributes. These were observability counters that the server bumps on every UI view/download, causing perpetual phantom drift on `terraform plan`. They have no declarative use case — if you need the numbers, fetch them with `curl` or the LangSmith SDK against `/api/v1/repos/-/{handle}`.
* `langsmith_prompt` data source: removed the same fields (`num_likes`, `num_commits`, `last_commit_hash`) for consistency.

  *Upgrade note:* existing state containing these attributes will be silently dropped on next `terraform plan`/`apply`; no migration required.

BUG FIXES:

* `langsmith_prompt` (resource): `owner` and `full_name` were never populated on Create/Read because the API nests them inside `repo` while the wire struct expected them at the top level. Fixed; `owner` may now be empty (the API returns `null` for service-account-created prompts), so all path construction falls back to `-` (current-workspace wildcard) when `owner` is unset.
* `langsmith_prompt` (resource): Update path could leave `manifest` as unknown after apply when the repo had no commits; now explicitly nulled.
* `langsmith_prompt` (data source): same nested-`repo` decoding bug as the resource — every read returned all-zero values. Fixed.
* `langsmith_insights_config`: server-injected nulls + default `filter` field in the `config` JSON caused "Provider produced inconsistent result after apply" errors. The plan's `config` value is now preserved across Create/Update/Read so server-side normalization is invisible to Terraform.
* `langsmith_evaluator`: removed `UseStateForUnknown` from `feedback_keys` (it's derived from `name`, so renaming was crashing with "inconsistent result").

INTERNAL / RESILIENCY:

* `internal/client`: `APIError` now carries the failing HTTP method and path; error messages read e.g. `LangSmith POST /v1/platform/evaluators returned 422: ...` instead of just `(status 422): ...`.
* `internal/client`: 408 (Request Timeout) is now treated as a transient retriable status alongside 429 and 5xx.
* `internal/client`: added 14 unit tests covering header injection, retry behavior on 408/429/5xx, `Retry-After` honoring, non-retriable 4xx fail-fast, context cancellation, body marshaling, and query encoding (this package previously had zero tests).
* `provider`: `api_url` trailing slashes are trimmed so self-hosted instances configured as `https://host/` no longer produce `//api/v1/info` requests.

FEATURES:

* **New Resource:** `langsmith_org_chart` - Organization-scoped custom charts (mirror of `langsmith_chart` at org scope)
* **New Resource:** `langsmith_org_chart_section` - Organization-scoped chart sections
* **New Resource:** `langsmith_evaluator` - Code and LLM-as-judge evaluators
* **New Resource:** `langsmith_gateway_policy` - LLM Gateway policies (spend caps, allow/deny)
* **New Resource:** `langsmith_tool` - Agent Builder platform-level tool definitions
* **New Resource:** `langsmith_hub_environment` - Prompt-hub environment list (1–4 named environments)
* **New Resource:** `langsmith_personal_access_token` - Org-scoped personal access tokens (create + delete only)
* **New Resource:** `langsmith_feedback_ingest_token` - Scoped feedback ingest tokens (no delete; tokens expire on their own)
* **New Resource:** `langsmith_dataset_share` - Public share state per dataset
* **New Resource:** `langsmith_dataset_split` - Named split membership within a dataset
* **New Resource:** `langsmith_annotation_queue_reviewer` - Reviewer membership on annotation queues
* **New Resource:** `langsmith_repo_owner` - Prompt-repo collaborators (added by email)
* **New Resource:** `langsmith_insights_config` - **Beta:** run-insights (clustering) job configs
* **New Data Source:** `langsmith_evaluator` - Look up an evaluator by ID
* **New Data Source:** `langsmith_tool` - Look up a tool by handle
* **New Data Source:** `langsmith_gateway_policy` - Look up a gateway policy by ID
* **New Data Source:** `langsmith_mcp_vendor` - Look up an MCP vendor by slug
* **New Data Source:** `langsmith_audit_log` - Page audit log entries in OCSF format
* **New Data Source:** `langsmith_data_planes` - List self-hosted data planes for the org

ENHANCEMENTS:

* `internal/client/client.go`: added `PutWithQuery` to support endpoints (like dataset share) that take query parameters on PUT
* `provider.Configure`: trailing slashes on `api_url` are now trimmed, preventing double-slash URLs against self-hosted instances

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

* Provider supports `workspace_id` for org-scoped API key authentication
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
