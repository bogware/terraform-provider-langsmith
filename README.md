<p align="center">
  <img src="https://img.shields.io/github/license/bogware/terraform-provider-langsmith?style=flat-square" alt="License">
  <img src="https://img.shields.io/github/v/release/bogware/terraform-provider-langsmith?style=flat-square" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/bogware/terraform-provider-langsmith/test.yml?branch=main&style=flat-square&label=tests" alt="Tests">
  <img src="https://img.shields.io/badge/terraform-%3E%3D1.0-blue?style=flat-square&logo=terraform" alt="Terraform">
</p>

# Terraform Provider for LangSmith

Manage your [LangSmith](https://smith.langchain.com/) infrastructure as code. This provider gives you full control over projects, datasets, annotation queues, prompts, automation rules, workspaces, and more through Terraform.

## Quick Start

```hcl
terraform {
  required_providers {
    langsmith = {
      source  = "bogware/langsmith"
      version = "~> 0.5"
    }
  }
}

provider "langsmith" {
  # API key: set here or via LANGSMITH_API_KEY env var
  # api_key = "lsv2_..."

  # Workspace ID: required for org-scoped keys
  # Set here or via LANGSMITH_TENANT_ID env var
  # tenant_id = "your-workspace-uuid"
}

# Create a tracing project
resource "langsmith_project" "production" {
  name        = "production"
  description = "Production LLM tracing"
}

# Create an evaluation dataset
resource "langsmith_dataset" "golden" {
  name        = "golden-dataset"
  description = "Curated examples for model evaluation"
  data_type   = "kv"
}

# Set up human review
resource "langsmith_annotation_queue" "review" {
  name                   = "human-review"
  description            = "Queue for reviewing flagged outputs"
  num_reviewers_per_item = 2
}

# Route errors to the review queue automatically
resource "langsmith_run_rule" "errors" {
  display_name               = "route-errors"
  sampling_rate              = 1.0
  session_id                 = langsmith_project.production.id
  filter                     = "eq(status, \"error\")"
  add_to_annotation_queue_id = langsmith_annotation_queue.review.id
}
```

## Authentication

| Method | Details |
|--------|---------|
| **Environment variable** (recommended) | `export LANGSMITH_API_KEY="lsv2_..."` |
| **Provider attribute** | `api_key = "lsv2_..."` |

### Org-Scoped API Keys

If you're using an organization-scoped service key, you **must** also provide your workspace ID:

| Method | Details |
|--------|---------|
| **Environment variable** | `export LANGSMITH_TENANT_ID="your-workspace-uuid"` |
| **Provider attribute** | `tenant_id = "your-workspace-uuid"` |

To find your workspace ID: **LangSmith Settings > Workspaces**, or:

```bash
curl -s -H "X-API-Key: $LANGSMITH_API_KEY" \
  https://api.smith.langchain.com/api/v1/workspaces | jq '.[].id'
```

### SSO (SAML) API surface

| LangSmith route | Terraform |
|-----------------|-----------|
| `GET`/`POST` `/api/v1/orgs/current/sso-settings`, `PATCH`/`DELETE` `/api/v1/orgs/current/sso-settings/{id}` | `langsmith_sso_settings` resource |
| `GET` `/api/v1/sso/settings/{sso_login_slug}` | `langsmith_sso_settings_by_slug` data source (read-only `SSOProviderSlim` list) |
| `POST` `/api/v1/sso/email-lookup`, `POST` `/api/v1/sso/email-verification/*` | Not supported (interactive / UI flows) |
| `POST` `/api/v1/orgs/current/set-default-sso-provision` | Not implemented |

### Feedback (`/api/v1/feedback*`)

| LangSmith route | Terraform |
|-----------------|-----------|
| `POST` `/api/v1/feedback/tokens`, `GET` `/api/v1/feedback/tokens?run_id=` | `langsmith_feedback_ingest_token` resource and `langsmith_feedback_ingest_tokens` data source |
| `GET` / `POST` `/api/v1/feedback/tokens/{token}` | Not supported (operational: submit feedback with a token) |
| `GET` / `POST` `/api/v1/feedback`, `POST` `/api/v1/feedback/eager`, `GET` / `PATCH` / `DELETE` `/api/v1/feedback/{feedback_id}` | Not supported (operational: scores on traces) |
| `GET` / `POST` / `PATCH` / `DELETE` `/api/v1/feedback-configs` | `langsmith_feedback_config` resource |
| `POST` / `GET` / `PUT` / `DELETE` `/api/v1/feedback/formulas` | `langsmith_feedback_formula` resource |

### Self-Hosted Instances

Override the API URL via `api_url` attribute or `LANGSMITH_API_URL` env var.

## Resources

| Resource | Description |
|----------|-------------|
| `langsmith_project` | Tracing projects (tracer sessions) |
| `langsmith_dataset` | Evaluation datasets |
| `langsmith_example` | Dataset examples (input/output pairs) |
| `langsmith_annotation_queue` | Annotation queues for human review |
| `langsmith_annotation_queue_reviewer` | Platform API reviewer membership on an annotation queue |
| `langsmith_service_account` | Service accounts (create + delete only) |
| `langsmith_service_key` | API service keys (create + delete only, key is sensitive) |
| `langsmith_api_key` | Tenant/workspace API keys via `/api/v1/api-key` (secret shown once at create) |
| `langsmith_prompt` | Prompts in the LangSmith Hub (with manifest/content management) |
| `langsmith_prompt_tag` | Named version tags on prompt commits (e.g., `production`, `staging`) |
| `langsmith_run_rule` | Automation rules for run routing |
| `langsmith_webhook` | Prompt webhooks |
| `langsmith_feedback_config` | Feedback score configurations |
| `langsmith_feedback_formula` | Computed feedback formulas (`/api/v1/feedback/formulas`) |
| `langsmith_feedback_ingest_token` | Feedback ingest tokens for external submission (`POST /api/v1/feedback/tokens`) |
| `langsmith_workspace` | Workspaces |
| `langsmith_tag_key` | Tag keys for resource tagging |
| `langsmith_tag_value` | Tag values (nested under tag keys) |
| `langsmith_bulk_export_destination` | Bulk export S3 destinations |
| `langsmith_bulk_export` | Bulk export jobs |
| `langsmith_model_price_map` | Model pricing configuration |
| `langsmith_usage_limit` | Usage limits |
| `langsmith_playground_settings` | Playground settings |
| `langsmith_secret` | Workspace secrets (key/value store) |
| `langsmith_ttl_settings` | Trace retention (TTL) settings |
| `langsmith_alert_rule` | Alert rules for project monitoring |
| `langsmith_org_role` | Organization roles (RBAC) |
| `langsmith_sso_settings` | SSO/SAML settings |
| `langsmith_workspace_member` | Workspace member management |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| `langsmith_project` | Look up a project by name or ID |
| `langsmith_dataset` | Look up a dataset by name or ID |
| `langsmith_workspace` | Look up a workspace by name or ID |
| `langsmith_info` | LangSmith server information |
| `langsmith_organization` | Current organization details |
| `langsmith_prompt_commit` | Read a specific prompt commit by hash, tag, or `latest` |
| `langsmith_tool` | Look up a platform registry tool by stable `handle` or `id` |
| `langsmith_sso_settings_by_slug` | Resolve SSO providers for a login slug (`GET /api/v1/sso/settings/{sso_login_slug}`) |
| `langsmith_feedback_ingest_tokens` | List feedback ingest tokens for a run (`GET /api/v1/feedback/tokens`) |

## Development

### Requirements

- [Go](https://golang.org/doc/install) >= 1.24
- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0

### Build & Test

```bash
make build        # Build the provider
make test         # Run unit tests (needs `terraform` on PATH; see note below)
make testacc      # Run acceptance tests (needs LANGSMITH_API_KEY + LANGSMITH_TENANT_ID)
make lint         # Run golangci-lint
make generate     # Regenerate docs from schemas + examples
```

`make test` uses [terraform-plugin-testing](https://developer.hashicorp.com/terraform/plugin/testing), which runs the Terraform CLI. If Terraform is missing, the helper may try to download a binary and fail with signature errors (e.g. `openpgp: key expired`); install Terraform or set **`TF_ACC_TERRAFORM_PATH`** to your `terraform` executable.

### Local Development

Add a dev override to `~/.terraformrc` to test without publishing:

```hcl
provider_installation {
  dev_overrides {
    "bogware/langsmith" = "/path/to/your/GOBIN"
  }
  direct {}
}
```

Then `make install` and use Terraform normally (skip `terraform init`).

### Running Acceptance Tests

Acceptance tests create real resources against the LangSmith API:

```bash
export LANGSMITH_API_KEY="lsv2_..."
export LANGSMITH_TENANT_ID="your-workspace-uuid"
make testacc
```

### Documentation

Docs in `docs/` are auto-generated from schemas and `examples/`. After modifying any resource schema or example config:

```bash
make generate
git add docs/
```

CI will fail if generated docs are stale.

## License

[MPL-2.0](LICENSE)
