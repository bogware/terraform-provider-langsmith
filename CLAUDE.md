# CLAUDE.md

## Project Overview

Terraform provider built on the **Terraform Plugin Framework** (not the legacy SDK v2). This is a scaffolding/template provider (`scaffolding`) that demonstrates all modern provider features: resources, data sources, ephemeral resources, functions, and actions. All provider logic lives in `internal/provider/`.

- **Go module**: `github.com/hashicorp/terraform-provider-scaffolding-framework`
- **Provider name**: `scaffolding`
- **Registry address**: `registry.terraform.io/hashicorp/scaffolding`
- **Protocol**: Terraform Plugin Protocol v6 only
- **License**: MPL-2.0

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

**Important**: After adding or modifying resources/data sources/functions, run `make generate` and commit the resulting changes to `docs/`. CI will fail if generated files are out of date.

## Project Structure

```
main.go                              # Entry point, provider server setup
internal/provider/
  provider.go                        # Provider definition, schema, Configure(), resource/datasource registration
  provider_test.go                   # Test helpers: provider factories, testAccPreCheck
  example_resource.go                # Resource implementation (CRUD + ImportState)
  example_resource_test.go           # Resource acceptance tests
  example_data_source.go             # Data source implementation (Read)
  example_data_source_test.go        # Data source acceptance tests
  example_ephemeral_resource.go      # Ephemeral resource implementation (Open)
  example_ephemeral_resource_test.go # Ephemeral resource acceptance tests
  example_function.go                # Provider-defined function implementation
  example_function_test.go           # Function unit tests
  example_action.go                  # Action implementation (Invoke)
  example_action_test.go             # Action acceptance tests
tools/tools.go                       # Code generation: copywrite headers, terraform fmt, tfplugindocs
examples/                            # Example .tf configs (used by doc generator)
docs/                                # Auto-generated documentation (do NOT edit manually)
```

## Code Conventions

### File Naming
- Implementation: `<name>_<type>.go` (e.g., `example_resource.go`, `example_data_source.go`)
- Tests: `<name>_<type>_test.go` (e.g., `example_resource_test.go`)

### Type Naming
- Resource struct: `ExampleResource` with model `ExampleResourceModel`
- Data source struct: `ExampleDataSource` with model `ExampleDataSourceModel`
- Constructor: `NewExampleResource() resource.Resource`
- Terraform type name: `req.ProviderTypeName + "_example"` (yields `scaffolding_example`)

### Interface Compliance
Every type must have a compile-time interface check at the top of the file:
```go
var _ resource.Resource = &ExampleResource{}
var _ resource.ResourceWithImportState = &ExampleResource{}
```

### Resource Implementation Pattern
Each resource/data source/action follows this structure:
1. Interface compliance var check
2. Constructor function (`New...()`)
3. Struct definition with client field
4. Model struct with `tfsdk:` tags (snake_case attribute names)
5. `Metadata()` - sets type name
6. `Schema()` - defines attributes with `MarkdownDescription`
7. `Configure()` - receives provider client from `req.ProviderData`
8. CRUD methods (Create/Read/Update/Delete) or Read (data sources) or Open (ephemeral) or Invoke (actions)

### Error Handling
- Use `resp.Diagnostics.AddError(summary, detail)` for errors
- Always check `resp.Diagnostics.HasError()` after reading plan/state/config
- Use `resp.Diagnostics.Append()` for propagating diagnostics from framework calls

### Logging
Use `tflog.Trace(ctx, "message")` from `github.com/hashicorp/terraform-plugin-log/tflog`.

### License Headers
All `.go` files must have the copyright header:
```go
// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0
```
Run `make generate` to auto-apply headers via copywrite.

## Testing

### Acceptance Tests
- Use `resource.Test(t, resource.TestCase{...})` with `testAccProtoV6ProviderFactories`
- For ephemeral resources, use `testAccProtoV6ProviderFactoriesWithEcho` (includes the echo provider)
- Test function names: `TestAcc<ResourceName>` (e.g., `TestAccExampleResource`)
- Config helpers: `testAcc<Resource>Config(...)` functions returning HCL strings
- State checks use `statecheck.ExpectKnownValue()` with `knownvalue` matchers and `tfjsonpath`
- Version-gated tests use `tfversion.SkipBelow()` for features requiring newer Terraform versions

### Running Tests
```bash
make test         # Unit tests only (no real infrastructure)
make testacc      # Full acceptance tests (creates real resources, needs TF_ACC=1)
```

CI runs acceptance tests against Terraform 1.13.x and 1.14.x.

## Linting

Configured via `.golangci.yml`. Key rules:
- **depguard**: Blocks imports from `terraform-plugin-sdk/v2` — use Plugin Framework equivalents instead
- Enabled linters: `errcheck`, `staticcheck`, `unused`, `misspell`, `forcetypeassert`, `usetesting`, among others
- Run with `make lint` or `golangci-lint run`

## Adding a New Resource

1. Create `internal/provider/<name>_resource.go` with the resource struct, model, and CRUD methods
2. Create `internal/provider/<name>_resource_test.go` with acceptance tests
3. Register in `provider.go` by adding to `Resources()` return slice
4. Add example config in `examples/resources/<provider>_<name>/resource.tf`
5. Run `make generate` to regenerate docs
6. Verify with `make lint && make test`

The same pattern applies for data sources (`DataSources()`), ephemeral resources (`EphemeralResources()`), functions (`Functions()`), and actions (`Actions()`).

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

## Common Pitfalls

- Never import `terraform-plugin-sdk/v2` — the linter will reject it. Use `terraform-plugin-framework` equivalents.
- Don't edit files in `docs/` manually — they are regenerated by `make generate` from schemas and `examples/`.
- Always run `make generate` before committing if you changed schemas or examples.
- Acceptance tests require `TF_ACC=1` environment variable to run.
- The provider address in `main.go` needs updating when forking this template for a real provider.
