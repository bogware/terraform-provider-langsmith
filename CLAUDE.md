# CLAUDE.md

## Project Overview

Terraform provider for [LangSmith](https://smith.langchain.com/) built on the **Terraform Plugin Framework** (not the legacy SDK v2). Manages LangSmith resources including projects, datasets, annotation queues, prompts, automation rules, and more via the [LangSmith REST API](https://api.smith.langchain.com/redoc).

- **Go module**: `github.com/bogware/terraform-provider-langsmith`
- **Provider name**: `langsmith`
- **Registry address**: `registry.terraform.io/bogware/langsmith`
- **Protocol**: Terraform Plugin Protocol v6 only
- **License**: MPL-2.0
- **LangSmith API**: `https://api.smith.langchain.com` (configurable for self-hosted)

## Build & Development Commands

```bash
make              # Runs: fmt, lint, install, generate (the default target)
make build        # go build -v ./...
make install      # go install -v ./...
make fmt          # gofmt -s -w -e .
make lint         # golangci-lint run
make generate     # Regenerate copyright headers, format examples, regenerate docs
make test         # Unit tests: go test -v -cover -timeout=120s -parallel=10 ./...
make testacc      # Acceptance tests: TF_ACC=1 go test -v -cover -timeout 120m ./...
```

**Important**: After adding or modifying resources/data sources, run `make generate` and commit the resulting changes to `docs/`. CI will fail if generated files are out of date.

## Project Structure

```
main.go                                  # Entry point, provider server setup (registry.terraform.io/bogware/langsmith)
internal/
  client/
    client.go                            # HTTP client for LangSmith API (auth, JSON, error handling)
  provider/
    provider.go                          # Provider definition: schema (api_key, api_url), Configure(), resource registration
    provider_test.go                     # Test helpers: provider factories, testAccPreCheck

    # Phase 1: Core Resources
    project_resource.go                  # langsmith_project (TracerSession CRUD)
    dataset_resource.go                  # langsmith_dataset (Dataset CRUD)
    example_resource.go                  # langsmith_example (Dataset example CRUD)
    annotation_queue_resource.go         # langsmith_annotation_queue (Annotation queue CRUD)
    service_account_resource.go          # langsmith_service_account (Create + Delete only)
    service_key_resource.go              # langsmith_service_key (Create + Delete only, key sensitive)

    # Phase 2: Prompts & Automation
    prompt_resource.go                   # langsmith_prompt (Hub repo CRUD, import via owner/handle)
    run_rule_resource.go                 # langsmith_run_rule (Automation rule CRUD)
    webhook_resource.go                  # langsmith_webhook (Prompt webhook CRUD)
    feedback_config_resource.go          # langsmith_feedback_config (Feedback config, keyed by feedback_key)

    # Phase 3: Workspace & Governance
    workspace_resource.go                # langsmith_workspace (Workspace CRUD)
    tag_key_resource.go                  # langsmith_tag_key (Tag key CRUD)
    tag_value_resource.go                # langsmith_tag_value (Tag value CRUD, nested under tag_key)

    # Phase 4: Export & Settings
    bulk_export_destination_resource.go  # langsmith_bulk_export_destination (S3 destination, no API delete)
    bulk_export_resource.go              # langsmith_bulk_export (Export job, cancel-as-delete)
    model_price_map_resource.go          # langsmith_model_price_map (Model pricing CRUD)
    usage_limit_resource.go              # langsmith_usage_limit (Usage limit upsert)
    playground_settings_resource.go      # langsmith_playground_settings (Playground settings CRUD)

    # Data Sources
    project_data_source.go               # langsmith_project (lookup by name or id)
    dataset_data_source.go               # langsmith_dataset (lookup by name or id)
    workspace_data_source.go             # langsmith_workspace (lookup by name or id)
    info_data_source.go                  # langsmith_info (server info, no inputs)

tools/tools.go                           # Code generation: copywrite headers, terraform fmt, tfplugindocs
examples/                                # Example .tf configs (used by doc generator)
docs/                                    # Auto-generated documentation (do NOT edit manually)
```

## Authentication

The provider authenticates via `x-api-key` header. Configuration:
- Provider attribute: `api_key` (sensitive)
- Environment variable: `LANGSMITH_API_KEY`
- API URL defaults to `https://api.smith.langchain.com`, override with `api_url` attribute or `LANGSMITH_API_URL` env var

## Code Conventions

### File Naming
- Resources: `<name>_resource.go` (e.g., `project_resource.go`, `dataset_resource.go`)
- Data sources: `<name>_data_source.go` (e.g., `project_data_source.go`)
- Tests: `<name>_<type>_test.go`

### Type Naming
- Resource struct: `ProjectResource` with model `ProjectResourceModel`
- Data source struct: `ProjectDataSource` with model `ProjectDataSourceModel`
- Constructor: `NewProjectResource() resource.Resource`
- Terraform type name: `req.ProviderTypeName + "_project"` (yields `langsmith_project`)
- API structs: unexported, e.g., `projectCreateRequest`, `projectAPIResponse`

### Interface Compliance
Every type must have a compile-time interface check at the top of the file:
```go
var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}
```

### Resource Implementation Pattern
Each resource follows this structure:
1. Interface compliance var check
2. Constructor function (`New...()`)
3. Struct definition with `client *client.Client` field
4. Terraform model struct with `tfsdk:` tags (snake_case attribute names)
5. Unexported API request/response structs for JSON marshaling
6. `Metadata()` - sets type name
7. `Schema()` - defines attributes with `MarkdownDescription`
8. `Configure()` - casts `req.ProviderData` to `*client.Client`
9. CRUD methods (Create/Read/Update/Delete)
10. `ImportState()` - usually `resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)`

### Client Usage
The `internal/client` package provides a generic HTTP client:
```go
client.Get(ctx, "/api/v1/sessions/UUID", nil, &result)
client.Post(ctx, "/api/v1/sessions", body, &result)
client.Patch(ctx, "/api/v1/sessions/UUID", body, &result)
client.Delete(ctx, "/api/v1/sessions/UUID")
```
- On 404, use `client.IsNotFound(err)` to check, then `resp.State.RemoveResource(ctx)`
- JSON fields that map to API objects (inputs, outputs, metadata, extra, settings) are stored as JSON strings in Terraform state

### Error Handling
- Use `resp.Diagnostics.AddError(summary, detail)` for errors
- Always check `resp.Diagnostics.HasError()` after reading plan/state/config
- Use `resp.Diagnostics.Append()` for propagating diagnostics from framework calls

### Logging
Use `tflog.Trace(ctx, "message")` from `github.com/hashicorp/terraform-plugin-log/tflog`.

### License Headers
All `.go` files must have the copyright header:
```go
// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0
```
Run `make generate` to auto-apply headers via copywrite.

## LangSmith API Reference

- **OpenAPI spec**: `https://api.smith.langchain.com/openapi.json` (697KB, comprehensive)
- **Redoc UI**: `https://api.smith.langchain.com/redoc`
- **Swagger UI**: `https://api.smith.langchain.com/docs`
- **Auth header**: `X-API-Key: <api_key>`
- **Key endpoint groups**: sessions, datasets, examples, annotation-queues, repos, runs/rules, prompt-webhooks, feedback-configs, workspaces, bulk-exports, model-price-map, service-accounts, orgs

## Testing

### Acceptance Tests
- Use `resource.Test(t, resource.TestCase{...})` with `testAccProtoV6ProviderFactories`
- Test function names: `TestAcc<ResourceName>` (e.g., `TestAccProjectResource`)
- Config helpers: `testAcc<Resource>Config(...)` functions returning HCL strings
- Requires `LANGSMITH_API_KEY` environment variable

### Running Tests
```bash
make test         # Unit tests only (no real infrastructure)
make testacc      # Full acceptance tests (creates real resources, needs TF_ACC=1)
```

## Adding a New Resource

1. Create `internal/provider/<name>_resource.go` with the resource struct, model, and CRUD methods
2. Create `internal/provider/<name>_resource_test.go` with acceptance tests
3. Register in `provider.go` by adding to `Resources()` return slice
4. Add example config in `examples/resources/langsmith_<name>/resource.tf`
5. Run `make generate` to regenerate docs
6. Verify with `make lint && make test`

The same pattern applies for data sources (`DataSources()`).

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `terraform-plugin-framework` v1.17.0 | Core provider SDK (schemas, CRUD, types) |
| `terraform-plugin-go` v0.29.0 | Low-level protocol implementation |
| `terraform-plugin-log` v0.10.0 | Structured logging (`tflog`) |
| `terraform-plugin-testing` v1.14.0 | Acceptance test framework |

## CI/CD

- **test.yml**: On push/PR — builds, lints, verifies `make generate` is clean, runs acceptance tests
- **release.yml**: On version tags (`v*`) — GoReleaser builds multi-platform binaries with GPG signing
- Go version is read from `go.mod` (`go 1.25.5`)
- Registry-compatible: semver tags, SHA256SUMS + GPG signature, multi-platform binaries

## Common Pitfalls

- Never import `terraform-plugin-sdk/v2` — the linter will reject it. Use `terraform-plugin-framework` equivalents.
- Don't edit files in `docs/` manually — they are regenerated by `make generate` from schemas and `examples/`.
- Always run `make generate` before committing if you changed schemas or examples.
- Acceptance tests require `TF_ACC=1` and `LANGSMITH_API_KEY` environment variables.
- Some resources (service_account, service_key) don't support update — they use RequiresReplace.
- The feedback_config resource uses `feedback_key` as its identifier, not a UUID.
- The prompt resource uses `owner/repo_handle` as its import ID format.
- The bulk_export_destination has no API delete endpoint — delete is a state-only removal.
- JSON string fields (inputs, outputs, metadata, settings, extra) must be marshaled/unmarshaled between Terraform strings and API objects.
