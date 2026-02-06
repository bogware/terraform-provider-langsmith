# Terraform Provider for LangSmith

The LangSmith Terraform provider allows you to manage [LangSmith](https://smith.langchain.com/) resources as infrastructure-as-code. LangSmith is an observability, evaluation, and deployment platform for LLM applications.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24 (to build the provider)
- A [LangSmith](https://smith.langchain.com/) account and API key

## Example Usage

```hcl
terraform {
  required_providers {
    langsmith = {
      source  = "bogware/langsmith"
      version = "~> 0.1"
    }
  }
}

provider "langsmith" {
  # Set via LANGSMITH_API_KEY environment variable, or:
  # api_key = "lsv2_..."

  # Required for org-scoped API keys:
  # tenant_id = "your-workspace-uuid"
  # Or set via LANGSMITH_TENANT_ID environment variable
}

# Create a project for tracing
resource "langsmith_project" "production" {
  name        = "production"
  description = "Production LLM tracing"
}

# Create a dataset for evaluation
resource "langsmith_dataset" "eval" {
  name        = "evaluation-set"
  description = "Golden dataset for model evaluation"
  data_type   = "kv"
}

# Create an annotation queue for human review
resource "langsmith_annotation_queue" "review" {
  name                   = "human-review"
  description            = "Queue for reviewing flagged outputs"
  num_reviewers_per_item = 2
}

# Set up an automation rule
resource "langsmith_run_rule" "sample" {
  display_name               = "sample-errors"
  sampling_rate              = 1.0
  session_id                 = langsmith_project.production.id
  filter                     = "eq(status, \"error\")"
  add_to_annotation_queue_id = langsmith_annotation_queue.review.id
}
```

## Authentication

The provider requires a LangSmith API key. You can provide it in two ways:

1. **Environment variable** (recommended): Set `LANGSMITH_API_KEY`
2. **Provider configuration**: Set the `api_key` attribute

For **org-scoped API keys**, you must also provide a tenant/workspace ID:

1. **Environment variable**: Set `LANGSMITH_TENANT_ID`
2. **Provider configuration**: Set the `tenant_id` attribute

For self-hosted LangSmith instances, set the `api_url` attribute or `LANGSMITH_API_URL` environment variable.

## Resources

| Resource | Description |
|----------|-------------|
| `langsmith_project` | Manage tracing projects (tracer sessions) |
| `langsmith_dataset` | Manage evaluation datasets |
| `langsmith_example` | Manage dataset examples |
| `langsmith_annotation_queue` | Manage annotation queues for human review |
| `langsmith_service_account` | Manage service accounts |
| `langsmith_service_key` | Manage API service keys |
| `langsmith_prompt` | Manage prompts in the LangSmith Hub |
| `langsmith_run_rule` | Manage automation rules for runs |
| `langsmith_webhook` | Manage prompt webhooks |
| `langsmith_feedback_config` | Manage feedback score configurations |
| `langsmith_workspace` | Manage workspaces |
| `langsmith_tag_key` | Manage tag keys |
| `langsmith_tag_value` | Manage tag values |
| `langsmith_bulk_export_destination` | Manage bulk export destinations (S3) |
| `langsmith_bulk_export` | Manage bulk export jobs |
| `langsmith_model_price_map` | Manage model pricing configuration |
| `langsmith_usage_limit` | Manage usage limits |
| `langsmith_playground_settings` | Manage playground settings |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| `langsmith_project` | Look up a project by name or ID |
| `langsmith_dataset` | Look up a dataset by name or ID |
| `langsmith_workspace` | Look up a workspace by name or ID |
| `langsmith_info` | Retrieve LangSmith server information |
| `langsmith_organization` | Retrieve current organization information |

## Developing the Provider

### Building

```shell
make build
```

### Testing

```shell
# Unit tests
make test

# Acceptance tests (requires API key and tenant ID)
export LANGSMITH_API_KEY="lsv2_..."
export LANGSMITH_TENANT_ID="your-workspace-uuid"
make testacc
```

### Generating Documentation

After changing any resource schemas or examples, regenerate the docs:

```shell
make generate
```

Commit the resulting changes in `docs/`. CI will fail if generated files are out of date.

### Local Development with Terraform

To test the provider locally without publishing, add a dev override to your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "bogware/langsmith" = "/path/to/go/bin"  # GOBIN path where 'go install' places binaries
  }
  direct {}
}
```

Then run `make install` to compile and install the provider binary, and use Terraform normally (skip `terraform init`).

## Publishing to the Terraform Registry

### Prerequisites

1. **GPG Key**: Generate a GPG key pair for signing releases:
   ```shell
   gpg --full-generate-key   # Choose RSA, 4096 bits
   gpg --armor --export "<key-email>"  # Get public key for registry
   ```

2. **GitHub Secrets**: Add these secrets to the GitHub repository:
   - `GPG_PRIVATE_KEY`: Your GPG private key (`gpg --armor --export-secret-keys "<key-id>"`)
   - `PASSPHRASE`: GPG key passphrase
   - `LANGSMITH_API_KEY`: API key for acceptance tests
   - `LANGSMITH_TENANT_ID`: Tenant ID for acceptance tests

3. **Terraform Registry Account**: Sign in at [registry.terraform.io](https://registry.terraform.io/) with your GitHub account.

### Creating a Release

1. **Ensure all tests pass**:
   ```shell
   make test
   make testacc
   ```

2. **Verify generated docs are up to date**:
   ```shell
   make generate
   git diff --exit-code  # Should show no changes
   ```

3. **Update CHANGELOG.md** with the release version and date.

4. **Tag the release** with a semver tag:
   ```shell
   git tag v0.1.0
   git push origin v0.1.0
   ```

5. The **Release workflow** will automatically:
   - Build multi-platform binaries (Linux, macOS, Windows / amd64, arm64, 386, arm)
   - Sign the SHA256SUMS with your GPG key
   - Create a GitHub release with all artifacts and the registry manifest

### Submitting to the Terraform Registry

1. Go to [registry.terraform.io/publish/provider](https://registry.terraform.io/publish/provider)
2. Select the `bogware/terraform-provider-langsmith` repository
3. Add your **GPG public key** (the one used for signing releases)
4. The registry will automatically detect your GitHub releases and publish new versions

### Release Checklist

- [ ] All acceptance tests pass
- [ ] `make generate` produces no diff
- [ ] CHANGELOG.md is updated
- [ ] Version tag follows semver (e.g., `v0.1.0`)
- [ ] GitHub release was created by the Release workflow
- [ ] Release contains: zip archives, SHA256SUMS, SHA256SUMS.sig, manifest.json
- [ ] GPG public key is registered at registry.terraform.io

### Required Repository Files for Registry

These files must exist and are already included in this repository:

| File | Purpose |
|------|---------|
| `terraform-registry-manifest.json` | Declares protocol version (v6) |
| `.goreleaser.yml` | Multi-platform build, signing, and release config |
| `docs/` | Auto-generated documentation (from `make generate`) |
| `examples/` | Example Terraform configs (used by doc generator) |

## License

MPL-2.0
