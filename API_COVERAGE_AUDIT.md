# LangSmith OpenAPI vs Terraform provider (audit 2026-05)

This document records the **2026-05-14** coverage audit for the LangSmith Terraform provider. It explains what the provider intentionally manages, what remains operational-only (**won’t do** for Terraform), and where to look in code and generated registry docs.

**Authoritative API reference:** [LangSmith OpenAPI (`openapi.json`)](https://api.smith.langchain.com/openapi.json) — paths under `/api/v1/*` and `/v1/platform/*`.

**Authoritative provider inventory:** `internal/provider/provider.go` (`Resources`, `DataSources`) and the [Terraform Registry documentation](https://registry.terraform.io/providers/bogware/langsmith/latest/docs).

## Audit gaps — resolution

Each row below was raised in the same audit batch. As of this document, each area is either **implemented** in the provider or **documented** with rationale (including explicit won’t-do).

| Topic | Provider / docs outcome |
|--------|-------------------------|
| API keys / PATs (`/api/v1/api-key`) | **Implemented:** `langsmith_api_key` — see `docs/resources/api_key.md`. |
| Audit logs | **Implemented:** `langsmith_audit_logs` — `docs/data-sources/audit_logs.md`. |
| Advanced org APIs | **Partially addressed:** `langsmith_organization`, `langsmith_org_member`, `langsmith_org_role` (+ data source) cover common org control-plane surfaces. Remaining list/pending/permission niches in OpenAPI are either read-heavy or UI-adjacent; extend case-by-case when a stable declarative contract exists. |
| Workspace settings | **Implemented:** `langsmith_settings` resource and data source — README table maps `/api/v1/settings*`. |
| Org charts | **Implemented:** `langsmith_org_chart`, `langsmith_org_chart_section` (requires `organization_id` / `LANGSMITH_ORGANIZATION_ID`). |
| Annotation queue reviewers | **Implemented:** `langsmith_annotation_queue_reviewer`. |
| Hosted evaluators | **Implemented:** `langsmith_evaluator` resource and `langsmith_evaluator` data source. |
| Org feature flags / model restrictions | **Implemented:** `langsmith_platform_feature` and `langsmith_platform_features` (bulk read). |
| Gateway policies | **Implemented:** `langsmith_gateway_policy`. |
| Fleet / MCP / GitHub App (`/v1/platform/fleet*`) | **Won’t do (for now):** No Terraform resources. These surfaces are integration and runtime connectivity (connectors, OAuth app flows, MCP bridges), not durable workspace/org configuration with stable idempotent lifecycle semantics suitable for IaC. Revisit if LangSmith exposes clearly versioned CRUD objects intended for automation. |
| Feedback tokens & eager feedback | **Documented:** README “Feedback” table — ingest token **lifecycle** is `langsmith_feedback_ingest_token` / `langsmith_feedback_ingest_tokens`; submitting scores and eager feedback remain operational trace APIs (see **Out of scope** below). |
| Tenants listing (`GET /api/v1/tenants`) | **Implemented with guardrails:** `langsmith_tenants` data source. Many workspace-scoped API keys receive **403** on this route; acceptance tests **skip** when the endpoint is forbidden — see `AGENTS.md` / `tenants_data_source` tests. |
| Platform tools registry | **Implemented:** `langsmith_tool` data source (`handle` or `id`). |
| Session agent versions | **Implemented:** `langsmith_project_agent_versions` (`session_id` = project UUID). |
| SSO coverage | **Documented:** README “SSO (SAML) API surface” maps `/api/v1/orgs/current/sso-settings`, `/api/v1/sso/settings/{slug}`, and explicitly lists interactive SSO routes as unsupported. |

## Explicitly out of scope for Terraform (won’t do — rationale)

These OpenAPI areas are **intentionally unmanaged**. They are operational, analytical, or end-user UX flows; they change at high frequency, lack idempotent lifecycle semantics, or belong in application code—not Terraform state.

| Area | Rationale |
|------|-----------|
| Trace/run **ingestion**, **query**, **streaming**, **stats** (`/runs`, run search, aggregates) | Hot path for observability data; semantics are query/analysis, not declarative infrastructure. |
| **Public share links** | Ephemeral sharing; security-sensitive; not org/workspace configuration. |
| **OAuth login** and interactive **SSO** steps (`/api/v1/sso/email-*`, etc.) | User authentication flows, not infra definitions (see README for supported SSO **settings** CRUD). |
| **Hub env**, **MCP proxy**, **ACE**, **comments/likes** on repos | Product/UX and collaboration surfaces; not stable control-plane resources for IaC. |
| **Feedback** on traces (`POST /api/v1/feedback`, `eager`, CRUD by `feedback_id`, token **submission** routes) | Runtime scores on spans; use ingest **tokens** for automation boundaries instead (`CHANGELOG.md` notes destroy does not revoke tokens server-side). |

## Maintainer note

When adding or removing resources, update **`internal/provider/provider.go`**, run **`make generate`**, and keep **`README.md`** resource/data-source tables and this file aligned so the audit stays truthful.
