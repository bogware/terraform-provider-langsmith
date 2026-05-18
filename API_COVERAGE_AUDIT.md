# LangSmith OpenAPI vs Terraform provider (audit 2026-05-18)

This document tracks provider coverage relative to the public LangSmith OpenAPI description. **Upstream baseline:** [bogware/terraform-provider-langsmith](https://github.com/bogware/terraform-provider-langsmith) (`upstream/main`). This fork keeps upstream types and APIs as-is, and adds only the **fork extensions** listed below where upstream has gaps.

**Method**

- **Upstream inventory:** `upstream/main` → `internal/provider/provider.go` — **47** resources and **25** data sources.
- **This fork:** `internal/provider/provider.go` — **51** resources and **34** data sources (upstream set + fork extensions; no duplicate Terraform types for the same endpoint).
- **OpenAPI reference:** [LangSmith `openapi.json`](https://api.smith.langchain.com/openapi.json) (OpenAPI **3.1.0**). Path count changes upstream over time.
- **Mapping:** Each Terraform type maps to explicit HTTP calls in `internal/provider/*.go`.

**Registry documentation:** [Terraform Registry — bogware/langsmith](https://registry.terraform.io/providers/bogware/langsmith/latest/docs).

## Fork extensions (not in upstream)

These exist only in this fork to fill coverage gaps. When upstream adds an equivalent type, prefer upstream naming and remove the fork duplicate.

| Kind | Terraform type | API / notes |
|------|----------------|-------------|
| Provider attr | `organization_id` | Sent as `X-Organization-Id`; required for many org-scoped calls upstream already exposes (org charts, audit logs, etc.). |
| Resource | `langsmith_api_key` | `GET`/`POST` `/api/v1/api-key`, `DELETE` `/api/v1/api-key/{id}` |
| Resource | `langsmith_settings` | `GET` `/api/v1/settings`, `POST` `/api/v1/settings/handle` |
| Resource | `langsmith_platform_feature` | `/v1/platform/features/*` (default/disabled models per feature) |
| Resource | `langsmith_fleet_mcp_server` | `/v1/platform/fleet/mcp-servers` |
| Data source | `langsmith_settings` | `GET` `/api/v1/settings` |
| Data source | `langsmith_tenants` | `GET` `/api/v1/tenants` (often 403 on workspace-scoped keys; tests skip) |
| Data source | `langsmith_organizations` | `GET` `/api/v1/orgs` |
| Data source | `langsmith_organization_permissions` | `GET` `/api/v1/orgs/permissions` |
| Data source | `langsmith_organization_pending_invites` | `GET` `/api/v1/orgs/pending` |
| Data source | `langsmith_platform_features` | `GET` `/v1/platform/features` |
| Data source | `langsmith_project_agent_versions` | `/v1/platform/sessions/{id}/agent-versions` |
| Data source | `langsmith_sso_settings_by_slug` | `GET` `/api/v1/sso/settings/{sso_login_slug}` |
| Data source | `langsmith_feedback_ingest_tokens` | `GET` `/api/v1/feedback/tokens` (lists tokens; upstream has create-only `langsmith_feedback_ingest_token` resource) |

**Removed fork duplicate:** `langsmith_audit_logs` — same endpoint as upstream `langsmith_audit_log` (`GET /api/v1/audit-logs`). Use `langsmith_audit_log` only.

## Control-plane coverage (by API theme)

Rows summarize whether Terraform exposes **durable** create/read/update/delete (where applicable) for that theme. Types marked *(fork)* appear only in this repository.

| Theme | Provider / docs outcome |
|--------|-------------------------|
| **Workspace / tenant** | **Implemented:** `langsmith_workspace`, `langsmith_settings` *(fork)* (+ data source), `langsmith_tenants` *(fork)*, `langsmith_workspace` (lookup), `langsmith_secret`, workspace tagging (`langsmith_tag_key`, `langsmith_tag_value`, `langsmith_tagging`), `langsmith_workspace_member`. |
| **API keys / PATs** | **Implemented:** `langsmith_api_key` *(fork)* (`/api/v1/api-key`). Service keys: `langsmith_service_key` (+ `langsmith_service_account`). PATs: `langsmith_personal_access_token`. |
| **Organization directory & RBAC** | **Implemented:** `langsmith_org_member`, `langsmith_org_role` (+ data source), `langsmith_access_policy`, `langsmith_scim_token`. **Read-only:** `langsmith_organization`, `langsmith_organizations` *(fork)*, `langsmith_organization_permissions` *(fork)*, `langsmith_organization_pending_invites` *(fork)*. Accept/decline invite flows remain **interactive / out of scope**. |
| **Advanced org APIs** | **Partially addressed:** Core membership, roles, SSO, SCIM, charts, platform, gateway, evaluators, and org lists above. Add one-off admin routes only when a stable declarative contract exists. |
| **Workspace / org settings & TTL** | **Implemented:** `langsmith_settings` *(fork)*, `langsmith_ttl_settings`, `langsmith_playground_settings`, `langsmith_usage_limit`, `langsmith_model_price_map`. |
| **Projects, datasets, examples** | **Implemented:** `langsmith_project` (+ data source), `langsmith_dataset` (+ data source), `langsmith_example`, `langsmith_dataset_share`, `langsmith_dataset_split`. |
| **Runs automation** | **Implemented:** `langsmith_run_rule` (+ data source), `langsmith_annotation_queue` (+ data source), `langsmith_annotation_queue_reviewer`, `langsmith_filter_view`, `langsmith_alert_rule`. |
| **Prompts & Hub** | **Implemented:** `langsmith_prompt` (+ data source), `langsmith_prompt_commit`, `langsmith_prompt_tag`, `langsmith_webhook`, `langsmith_repo_owner`, `langsmith_hub_environment`. |
| **Feedback configuration** | **Implemented:** `langsmith_feedback_config`, `langsmith_feedback_formula`, `langsmith_feedback_ingest_token`, `langsmith_feedback_ingest_tokens` *(fork)*. **Not Terraform:** posting scores on traces — see **Out of scope**. |
| **Charts** | **Implemented:** workspace `langsmith_chart`, `langsmith_chart_section`, `langsmith_chart_section_clone`; org `langsmith_org_chart`, `langsmith_org_chart_section`; lookup/preview data sources. |
| **SSO (SAML)** | **Implemented:** `langsmith_sso_settings`; **read-only:** `langsmith_sso_settings_by_slug` *(fork)*. Interactive `/api/v1/sso/email-*` flows are **out of scope**. |
| **Audit** | **Implemented:** `langsmith_audit_log` (`GET /api/v1/audit-logs`). |
| **Bulk export** | **Implemented:** `langsmith_bulk_export_destination`, `langsmith_bulk_export`. |
| **Platform — tools** | **Implemented:** `langsmith_tool` (resource + data source, `/v1/platform/tools`). |
| **Platform — agent versions** | **Read-only:** `langsmith_project_agent_versions` *(fork)*. |
| **Platform — features / evaluators / gateway** | **Implemented:** `langsmith_platform_feature` *(fork)*, `langsmith_platform_features` *(fork)*, `langsmith_evaluator` (+ data source), `langsmith_gateway_policy` (+ data source). |
| **Platform — fleet MCP** | **Implemented:** `langsmith_fleet_mcp_server` *(fork)*. Other fleet GitHub App / usage / webhook routes: **won’t do** (see below). |
| **Tenants listing** | **Implemented with guardrails:** `langsmith_tenants` *(fork)*. Workspace-scoped keys often **403** — acceptance tests skip (`AGENTS.md`). |
| **Info / users / data planes** | **Read-only:** `langsmith_info`, `langsmith_user`, `langsmith_data_planes`. |
| **Insights (beta)** | **Implemented:** `langsmith_insights_config`. |

## Explicitly out of scope for Terraform (won’t do — rationale)

| Area | Rationale |
|------|-----------|
| Trace/run **ingestion**, **query**, **streaming**, **stats** | Hot path; query/analysis semantics, not declarative infra. |
| **Public share links** (beyond dataset share state) | Ephemeral sharing; security-sensitive. |
| **OAuth login** and interactive **SSO** steps | User auth flows, not infra definitions. |
| **MCP proxy**, **ACE**, **comments/likes** on repos | Product/UX surfaces. |
| **Feedback** on traces (`POST /api/v1/feedback`, eager, etc.) | Runtime scores; use ingest **tokens** for automation boundaries. |
| **Fleet** GitHub App / usage / webhook delivery | Connector and telemetry; not steady-state IaC. |

## Maintainer note

When adding or removing resources, compare against **`upstream/main`** first. If upstream already ships the type, do not add a fork duplicate. Update **`internal/provider/provider.go`**, run **`make generate`**, and keep **`README.md`**, **this file**, and the **Fork extensions** table aligned.
