# LangSmith OpenAPI vs Terraform provider (audit 2026-05)

This document closes the **2026-05-14** coverage audit tracked in Linear **[TYTY-79](https://linear.app/team-tyty/issue/TYTY-79/meta-langsmith-api-vs-terraform-provider-coverage-audit-2026-05)**. It records what the provider intentionally manages, what remains operational-only (**won’t do** for Terraform), and where to look in code and generated registry docs.

**Authoritative API reference:** [LangSmith OpenAPI (`openapi.json`)](https://api.smith.langchain.com/openapi.json) — paths under `/api/v1/*` and `/v1/platform/*`.

**Authoritative provider inventory:** `internal/provider/provider.go` (`Resources`, `DataSources`) and the [Terraform Registry documentation](https://registry.terraform.io/providers/bogware/langsmith/latest/docs).

## Child gap tickets (TYTY-64–TYTY-78) — resolution

These issues were filed from the same audit batch. As of this document, each is either **implemented** in the provider or **documented** below with rationale (including explicit won’t-do).

| Linear | Topic | Provider / docs outcome |
|--------|--------|-------------------------|
| [TYTY-64](https://linear.app/team-tyty/issue/TYTY-64/add-terraform-support-for-api-keys-pats-apiv1api-key) | API keys / PATs (`/api/v1/api-key`) | **Implemented:** `langsmith_api_key` — see `docs/resources/api_key.md`. |
| [TYTY-65](https://linear.app/team-tyty/issue/TYTY-65/add-data-source-for-audit-logs-apiv1audit-logs) | Audit logs | **Implemented:** `langsmith_audit_logs` — `docs/data-sources/audit_logs.md`. |
| [TYTY-66](https://linear.app/team-tyty/issue/TYTY-66/cover-advanced-organization-apis-orgs-list-pending-permissions) | Advanced org APIs | **Partially addressed:** `langsmith_organization`, `langsmith_org_member`, `langsmith_org_role` (+ data source) cover common org control-plane surfaces. Remaining list/pending/permission niches in OpenAPI are either read-heavy or UI-adjacent; extend case-by-case when a stable declarative contract exists. |
| [TYTY-67](https://linear.app/team-tyty/issue/TYTY-67/add-workspaceuser-settings-apiv1settings) | Workspace settings | **Implemented:** `langsmith_settings` resource and data source — README table maps `/api/v1/settings*`. |
| [TYTY-68](https://linear.app/team-tyty/issue/TYTY-68/add-org-level-charts-openapi-apiv1org-charts) | Org charts | **Implemented:** `langsmith_org_chart`, `langsmith_org_chart_section` (requires `organization_id` / `LANGSMITH_ORGANIZATION_ID`). |
| [TYTY-69](https://linear.app/team-tyty/issue/TYTY-69/support-annotation-queue-reviewers-platform-api) | Annotation queue reviewers | **Implemented:** `langsmith_annotation_queue_reviewer`. |
| [TYTY-70](https://linear.app/team-tyty/issue/TYTY-70/add-hosted-evaluators-resource-v1platformevaluators) | Hosted evaluators | **Implemented:** `langsmith_evaluator` resource and `langsmith_evaluator` data source. |
| [TYTY-71](https://linear.app/team-tyty/issue/TYTY-71/add-org-feature-flags-and-model-restrictions-v1platformfeatures) | Org feature flags / model restrictions | **Implemented:** `langsmith_platform_feature` and `langsmith_platform_features` (bulk read). |
| [TYTY-72](https://linear.app/team-tyty/issue/TYTY-72/add-gateway-policies-resource-v1platformgateway-policies) | Gateway policies | **Implemented:** `langsmith_gateway_policy`. |
| [TYTY-73](https://linear.app/team-tyty/issue/TYTY-73/evaluate-fleet-mcp-github-app-terraform-coverage-v1platformfleet) | Fleet / MCP / GitHub App (`/v1/platform/fleet*`) | **Won’t do (for now):** No Terraform resources. These surfaces are integration and runtime connectivity (connectors, OAuth app flows, MCP bridges), not durable workspace/org configuration with stable idempotent lifecycle semantics suitable for IaC. Revisit if LangSmith exposes clearly versioned CRUD objects intended for automation. |
| [TYTY-74](https://linear.app/team-tyty/issue/TYTY-74/clarify-coverage-for-feedback-tokens-and-eager-feedback-apis) | Feedback tokens & eager feedback | **Documented:** README “Feedback” table — ingest token **lifecycle** is `langsmith_feedback_ingest_token` / `langsmith_feedback_ingest_tokens`; submitting scores and eager feedback remain operational trace APIs (see **Out of scope** below). |
| [TYTY-75](https://linear.app/team-tyty/issue/TYTY-75/add-or-document-tenants-listing-apiv1tenants) | Tenants listing (`GET /api/v1/tenants`) | **Implemented with guardrails:** `langsmith_tenants` data source. Many workspace-scoped API keys receive **403** on this route; acceptance tests **skip** when the endpoint is forbidden — see `AGENTS.md` / `tenants_data_source` tests. |
| [TYTY-76](https://linear.app/team-tyty/issue/TYTY-76/data-source-platform-tools-registry-langsmith-tool) | Platform tools registry | **Implemented:** `langsmith_tool` data source (`handle` or `id`). |
| [TYTY-77](https://linear.app/team-tyty/issue/TYTY-77/add-data-source-for-session-agent-versions-platform-api) | Session agent versions | **Implemented:** `langsmith_project_agent_versions` (`session_id` = project UUID). |
| [TYTY-78](https://linear.app/team-tyty/issue/TYTY-78/reconcile-sso-terraform-coverage-with-apiv1sso-vs-orgscurrentsso) | SSO coverage | **Documented:** README “SSO (SAML) API surface” maps `/api/v1/orgs/current/sso-settings`, `/api/v1/sso/settings/{slug}`, and explicitly lists interactive SSO routes as unsupported. |

Related CI/testing meta: [TYTY-92](https://linear.app/team-tyty/issue/TYTY-92/acceptance-level-tests-without-fork-secrets-live-saas-contract-backed-ci) (contract-backed tests) is tracked separately from API surface coverage.

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
