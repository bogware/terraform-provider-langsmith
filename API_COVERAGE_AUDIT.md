# LangSmith OpenAPI vs Terraform provider (audit 2026-05-18)

This document is a **fresh pass** over provider coverage relative to the public LangSmith OpenAPI description. It replaces the May 2026 snapshot for day-to-day accuracy; historical gap-tracking language has been folded into the tables below.

**Method**

- **Provider inventory:** `internal/provider/provider.go` — **43** resources and **25** data sources in the registry at the time of this audit.
- **OpenAPI reference:** [LangSmith `openapi.json`](https://api.smith.langchain.com/openapi.json) (OpenAPI **3.1.0**). The published document contained **389** path templates when checked for this review (ordering and count can change upstream).
- **Mapping:** Each Terraform type is backed by explicit HTTP calls in `internal/provider/*.go`; this file summarizes outcomes by **API theme**, not by enumerating every path.

**Registry documentation:** [Terraform Registry — bogware/langsmith](https://registry.terraform.io/providers/bogware/langsmith/latest/docs).

## Control-plane coverage (by API theme)

Rows summarize whether Terraform exposes **durable** create/read/update/delete (where applicable) for that theme. Gaps call out read-only support, interactive-only API leftovers, or deliberate “won’t do”.

| Theme | Provider / docs outcome |
|--------|-------------------------|
| **Workspace / tenant** | **Implemented:** `langsmith_workspace`, `langsmith_settings` (+ data source), `langsmith_tenants`, `langsmith_workspace` (lookup), `langsmith_secret`, workspace tagging (`langsmith_tag_key`, `langsmith_tag_value`, `langsmith_tagging`), `langsmith_workspace_member`. |
| **API keys / PATs** (`/api/v1/api-key*`) | **Implemented:** `langsmith_api_key`. Service keys: `langsmith_service_key` (+ accounts `langsmith_service_account`). |
| **Organization directory & RBAC** | **Implemented:** `langsmith_org_member`, `langsmith_org_role` (+ data source), `langsmith_access_policy`, `langsmith_scim_token`. **Read-only list/catalog:** `langsmith_organization`, `langsmith_organizations` (`GET /api/v1/orgs`), `langsmith_organization_permissions` (`GET /api/v1/orgs/permissions`), `langsmith_organization_pending_invites` (`GET /api/v1/orgs/pending`). Accept/decline invite flows under `/api/v1/orgs/pending/...` remain **interactive / out of scope** — see data source docs. |
| **Advanced org APIs** | **Partially addressed:** Core membership, roles, SSO, SCIM, charts, platform features, gateway, evaluators, and org lists above cover most control-plane needs. **Remaining** org routes in OpenAPI (one-off admin or high-churn surfaces) should be added only when a stable declarative contract exists. |
| **Workspace / org settings & TTL** | **Implemented:** `langsmith_settings`, `langsmith_ttl_settings` (`/api/v1/orgs/ttl-settings`), `langsmith_playground_settings`, `langsmith_usage_limit`, `langsmith_model_price_map`. |
| **Projects, datasets, examples** | **Implemented:** `langsmith_project` (+ data source), `langsmith_dataset` (+ data source), `langsmith_example`. |
| **Runs automation** | **Implemented:** `langsmith_run_rule` (+ data source), `langsmith_annotation_queue` (+ data source), `langsmith_annotation_queue_reviewer`, `langsmith_filter_view`, `langsmith_alert_rule`. |
| **Prompts & Hub** | **Implemented:** `langsmith_prompt` (+ data source), `langsmith_prompt_commit`, `langsmith_prompt_tag`, `langsmith_webhook`. |
| **Feedback configuration** | **Implemented:** `langsmith_feedback_config`, `langsmith_feedback_formula`, ingest token **lifecycle:** `langsmith_feedback_ingest_token`, `langsmith_feedback_ingest_tokens`. **Not Terraform:** posting scores / eager feedback on traces (operational APIs) — see **Out of scope**. |
| **Charts** | **Implemented:** project `langsmith_chart`, `langsmith_chart_section`; org `langsmith_org_chart`, `langsmith_org_chart_section`. Bulk chart reads / preview endpoints are **not** modeled as separate resources (see resource descriptions). |
| **SSO (SAML)** | **Implemented:** `langsmith_sso_settings`; **read-only:** `langsmith_sso_settings_by_slug`. Interactive email discovery / verification under `/api/v1/sso/email-*` are **out of scope**. README “SSO (SAML) API surface” lists supported paths. |
| **Audit** | **Implemented:** `langsmith_audit_logs` (`GET /api/v1/audit-logs`). |
| **Bulk export** | **Implemented:** `langsmith_bulk_export_destination`, `langsmith_bulk_export`. |
| **Platform — tools** | **Read-only:** `langsmith_tool` (`/v1/platform/tools/...`). |
| **Platform — agent versions** | **Read-only:** `langsmith_project_agent_versions` (`/v1/platform/sessions/{id}/agent-versions`). |
| **Platform — features / evaluators / gateway** | **Implemented:** `langsmith_platform_feature`, `langsmith_platform_features`, `langsmith_evaluator` (+ data source), `langsmith_gateway_policy`. |
| **Platform — fleet MCP servers** | **Implemented:** `langsmith_fleet_mcp_server` (`POST/GET/PATCH/DELETE /v1/platform/fleet/mcp-servers` and `.../{id}`). OAuth-provider sub-resource route in OpenAPI is **not** a separate Terraform resource (handled via resource attributes where applicable). |
| **Platform — fleet (remainder)** | **Won’t do (for now):** GitHub App installation/connect/token/webhook routes under `/v1/platform/fleet/providers/github-app/...`, fleet **usage** aggregates (`/v1/platform/fleet/usage/...`), and **fleet webhook run** delivery paths. These are connector, OAuth, metering, or event-delivery surfaces rather than steady-state org/workspace config with uniform idempotent lifecycle semantics. **Revisit** if LangSmith documents first-class CRUD objects meant for IaC. |
| **Tenants listing** | **Implemented with guardrails:** `langsmith_tenants` (`GET /api/v1/tenants`). Workspace-scoped keys often receive **403** — acceptance tests **skip** when forbidden (`AGENTS.md`). |
| **Info / users** | **Read-only:** `langsmith_info`, `langsmith_user`. |

## Explicitly out of scope for Terraform (won’t do — rationale)

These OpenAPI areas are **intentionally unmanaged**. They are operational, analytical, or end-user UX flows; they change at high frequency, lack idempotent lifecycle semantics, or belong in application code—not Terraform state.

| Area | Rationale |
|------|-----------|
| Trace/run **ingestion**, **query**, **streaming**, **stats** (`/runs`, run search, aggregates) | Hot path for observability data; semantics are query/analysis, not declarative infrastructure. |
| **Public share links** | Ephemeral sharing; security-sensitive; not org/workspace configuration. |
| **OAuth login** and interactive **SSO** steps (`/api/v1/sso/email-*`, etc.) | User authentication flows, not infra definitions (see README for supported SSO **settings** CRUD). |
| **Hub env**, **MCP proxy**, **ACE**, **comments/likes** on repos | Product/UX and collaboration surfaces; not stable control-plane resources for IaC. |
| **Feedback** on traces (`POST /api/v1/feedback`, `eager`, CRUD by `feedback_id`, token **submission** routes) | Runtime scores on spans; use ingest **tokens** for automation boundaries instead (`CHANGELOG.md` notes destroy does not revoke tokens server-side). |
| **Fleet** GitHub App / usage / webhook delivery (see table above) | Connector and telemetry flows; not mapped to Terraform resources in this provider. |

## Maintainer note

When adding or removing resources, update **`internal/provider/provider.go`**, run **`make generate`**, and keep **`README.md`** resource/data-source tables and **this file** aligned so the audit stays truthful.
