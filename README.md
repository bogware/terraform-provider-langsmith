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
      version = "~> 0.9"
    }
  }
}

provider "langsmith" {
  # API key: set here or via LANGSMITH_API_KEY env var
  # api_key = "lsv2_..."

  # Workspace ID: required for org-scoped keys
  # Set here or via LANGSMITH_WORKSPACE_ID env var
  # workspace_id = "your-workspace-uuid"
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
| **Environment variable** | `export LANGSMITH_WORKSPACE_ID="your-workspace-uuid"` |
| **Provider attribute** | `workspace_id = "your-workspace-uuid"` |

To find your workspace ID: **LangSmith Settings > Workspaces**, or:

```bash
curl -s -H "X-API-Key: $LANGSMITH_API_KEY" \
  https://api.smith.langchain.com/api/v1/workspaces | jq '.[].id'
```

### Self-Hosted Instances

Override the API URL via `api_url` attribute or `LANGSMITH_API_URL` env var.

### Managing multiple workspaces

`workspace_id` is configured at the provider level, but you can use multiple provider instances with aliases to manage resources across different workspaces. For example:

```hcl
provider "langsmith" {
  alias        = "prod"
  workspace_id = "00000000-0000-0000-0000-prod"
}

provider "langsmith" {
  alias        = "staging"
  workspace_id = "00000000-0000-0000-0000-stg"
}

resource "langsmith_project" "prod_traces" {
  provider = langsmith.prod
  name     = "production"
}

resource "langsmith_project" "staging_traces" {
  provider = langsmith.staging
  name     = "staging"
}
```

This pattern works well for a known, static set of workspaces. For dynamic management of every workspace in the organization, combine the `langsmith_workspaces` data source with `for_each` and resource-level `workspace_id` ([issue #21](https://github.com/bogware/terraform-provider-langsmith/issues/21)):

```hcl
data "langsmith_workspaces" "all" {}

resource "langsmith_project" "tracing" {
  for_each = { for w in data.langsmith_workspaces.all.workspaces : w.id => w }

  name         = "tracing"
  workspace_id = each.key
}
```

You can also specify the workspace at the resource level for most resources, which allows you to manage multiple workspaces without multiple provider instances. For example:

```hcl
resource "langsmith_project" "prod_traces" {
  name       = "production"
  workspace_id = "00000000-0000-0000-0000-prod"
}

resource "langsmith_project" "staging_traces" {
  name       = "staging"
  workspace_id = "00000000-0000-0000-0000-stg"
}
```

## Resources

### Projects, datasets, examples

| Resource | Description |
|----------|-------------|
| `langsmith_project` | Tracing projects (tracer sessions) |
| `langsmith_dataset` | Evaluation datasets |
| `langsmith_example` | Dataset examples (input/output pairs) |
| `langsmith_dataset_share` | Public share state per dataset |
| `langsmith_dataset_split` | Named split membership within a dataset |
| `langsmith_dataset_version_tag` | Named tags on dataset versions (pin `prod` to a snapshot) |
| `langsmith_experiment_view_override` | Per-dataset experiment view column configuration |
| `langsmith_run_share` | Public share state for a run |

### Prompts (LangSmith Hub)

| Resource | Description |
|----------|-------------|
| `langsmith_prompt` | Prompts in the LangSmith Hub (with manifest/content management) |
| `langsmith_prompt_tag` | Named version tags on prompt commits (e.g., `production`, `staging`) |
| `langsmith_repo_owner` | Prompt-repo collaborators (added by email) |
| `langsmith_hub_environment` | Prompt-hub environment list (1–4 named environments) |

### Annotation, feedback, evaluation

| Resource | Description |
|----------|-------------|
| `langsmith_annotation_queue` | Annotation queues for human review |
| `langsmith_annotation_queue_reviewer` | Add/remove a reviewer identity on a queue |
| `langsmith_feedback_config` | Feedback score configurations |
| `langsmith_feedback_formula` | Derived-feedback formulas |
| `langsmith_feedback_ingest_token` | Run-scoped feedback ingest tokens (create-only; expire naturally) |
| `langsmith_evaluator` | Code and LLM-as-judge evaluators |
| `langsmith_run_rule` | Automation rules for run routing |
| `langsmith_filter_view` | Saved filter views on a tracing project |

### Charts and dashboards

| Resource | Description |
|----------|-------------|
| `langsmith_chart` | Workspace-scoped custom charts |
| `langsmith_chart_section` | Workspace-scoped chart sections |
| `langsmith_chart_section_clone` | Clone an existing chart section |
| `langsmith_org_chart` | Organization-scoped custom charts |
| `langsmith_org_chart_section` | Organization-scoped chart sections |
| `langsmith_insights_config` | **Beta:** run-insights (clustering) job configs |

### Workspaces, tagging, secrets

| Resource | Description |
|----------|-------------|
| `langsmith_workspace` | Workspaces |
| `langsmith_workspace_member` | Workspace member management |
| `langsmith_tag_key` | Tag keys for resource tagging |
| `langsmith_tag_value` | Tag values (nested under tag keys) |
| `langsmith_tagging` | Assign a tag value to a resource |
| `langsmith_secret` | Workspace secrets (key/value store) |
| `langsmith_ttl_settings` | Trace retention (TTL) settings |
| `langsmith_usage_limit` | Usage limits |
| `langsmith_workspace_handle` | Workspace hub handle (set-only) |
| `langsmith_feature_model_config` | Default/disabled models per platform feature |

### Org / identity / access

| Resource | Description |
|----------|-------------|
| `langsmith_service_account` | Service accounts (create + delete only) |
| `langsmith_service_key` | API service keys (create + delete only, key is sensitive) |
| `langsmith_personal_access_token` | Org-scoped personal access tokens (create + delete only) |
| `langsmith_org_role` | Organization roles (RBAC) |
| `langsmith_org_member` | Organization members |
| `langsmith_sso_settings` | SSO/SAML settings |
| `langsmith_access_policy` | Access policies (RBAC bindings) |
| `langsmith_scim_token` | SCIM provisioning tokens |
| `langsmith_organization_settings` | Current-organization settings (name, login methods, security toggles) |
| `langsmith_data_plane` | Self-hosted data planes (create-only) |

### Integrations, gateway, tools

| Resource | Description |
|----------|-------------|
| `langsmith_webhook` | Prompt webhooks |
| `langsmith_alert_rule` | Alert rules for project monitoring |
| `langsmith_gateway_policy` | LLM Gateway policies (spend caps, allow/deny) |
| `langsmith_tool` | Agent Builder platform-level tool definitions |
| `langsmith_playground_settings` | Playground settings |
| `langsmith_model_price_map` | Model pricing configuration |
| `langsmith_bulk_export_destination` | Bulk export S3 destinations |
| `langsmith_bulk_export` | Bulk export jobs |
| `langsmith_agent_builder_integrations` | Workspace Agent Builder integrations settings |
| `langsmith_mcp_vendor_settings` | MCP vendor settings (org/project bindings) |
| `langsmith_issues_agent` | **Beta:** per-project issues agent configuration |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| `langsmith_info` | LangSmith server information |
| `langsmith_organization` | Current organization details |
| `langsmith_workspace` | Look up a workspace by name or ID |
| `langsmith_user` | Look up a user by email |
| `langsmith_project` | Look up a project by name or ID |
| `langsmith_dataset` | Look up a dataset by name or ID |
| `langsmith_annotation_queue` | Look up an annotation queue by name or ID |
| `langsmith_prompt` | Look up a prompt repo by handle |
| `langsmith_prompt_commit` | Read a specific prompt commit by hash, tag, or `latest` |
| `langsmith_run_rule` | Look up a run rule by ID |
| `langsmith_service_account` | Look up a service account by name or ID |
| `langsmith_org_role` | Look up an org role by name or ID |
| `langsmith_tag_key` | Look up a tag key |
| `langsmith_evaluator` | Look up an evaluator by ID |
| `langsmith_tool` | Look up a platform tool by handle |
| `langsmith_gateway_policy` | Look up a gateway policy by ID |
| `langsmith_mcp_vendor` | Look up an MCP vendor by slug |
| `langsmith_audit_log` | Page audit log entries (OCSF format) |
| `langsmith_data_planes` | List self-hosted data planes for the org |
| `langsmith_chart` / `langsmith_chart_section` | Look up workspace charts and sections |
| `langsmith_org_chart` / `langsmith_org_chart_section` | Look up org-scoped charts and sections |
| `langsmith_chart_preview` / `langsmith_org_chart_preview` | Preview chart data points |
| `langsmith_workspaces` | List all workspaces (enables `for_each` multi-workspace management) |
| `langsmith_projects` / `langsmith_datasets` / `langsmith_prompts` | Filterable plural listings |
| `langsmith_evaluators` / `langsmith_run_rules` | Filterable plural listings |
| `langsmith_playground_settings` / `langsmith_usage_limits` / `langsmith_hub_environments` | Workspace-level listings |
| `langsmith_workspace_members` / `langsmith_org_members` | Member and pending-invite listings |
| `langsmith_permissions` | Org permission catalog (for authoring `langsmith_org_role`) |
| `langsmith_secret_names` | List workspace secret names (values are never returned) |
| `langsmith_example` | Read a dataset example by ID |
| `langsmith_feedback_config` | Look up a feedback config by key |
| `langsmith_filter_view` | Read a saved filter view |
| `langsmith_tag_value` | Look up a tag value by ID or value |
| `langsmith_bulk_export` | Read a bulk export job by ID |
| `langsmith_sso_settings` | Read the org's SSO/SAML settings |
| `langsmith_workspace_stats` | Workspace object counts |
| `langsmith_org_usage` | Organization usage over a date range |
| `langsmith_evaluator_spend` | Evaluator spend |
| `langsmith_issues` | **Beta:** list detected issues, optionally per project |

## Development

### Requirements

- [Go](https://golang.org/doc/install) >= 1.24
- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0

### Build & Test

```bash
make build        # Build the provider
make test         # Run unit tests
make testacc      # Run acceptance tests (needs LANGSMITH_API_KEY + LANGSMITH_WORKSPACE_ID)
make lint         # Run golangci-lint
make generate     # Regenerate docs from schemas + examples
```

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
export LANGSMITH_WORKSPACE_ID="your-workspace-uuid"
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
