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
      source = "bogware/langsmith"
    }
  }
}

provider "langsmith" {
  # Set via LANGSMITH_API_KEY environment variable, or:
  # api_key = "lsv2_..."
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

## Developing the Provider

### Building

```shell
make build
```

### Testing

```shell
# Unit tests
make test

# Acceptance tests (requires LANGSMITH_API_KEY)
make testacc
```

### Generating Documentation

```shell
make generate
```

## License

MPL-2.0
