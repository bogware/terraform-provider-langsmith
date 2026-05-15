# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common commands

All workflow commands live in `GNUmakefile`:

- `make build` — compile the provider
- `make install` — install the provider binary into `GOBIN` (used together with a `~/.terraformrc` dev override; see README)
- `make test` — unit tests (`go test -v -cover -timeout=120s -parallel=10 ./...`)
- `make testacc` — acceptance tests against the live LangSmith API; requires `LANGSMITH_API_KEY` and (for org-scoped keys) `LANGSMITH_TENANT_ID`. Sets `TF_ACC=1`.
- `make lint` — `golangci-lint run`
- `make fmt` — `gofmt -s -w -e .`
- `make generate` — regenerates `docs/` and re-formats `examples/`. Runs `tools/tools.go` go:generate directives (copywrite headers, `terraform fmt -recursive ../examples/`, `tfplugindocs generate`). CI fails if `docs/` is stale, so run this after any schema or example change and commit the result.

Single test: `go test ./internal/provider -run TestName -v`. Acceptance variants are gated by `TF_ACC=1` and `testAccPreCheck` (which requires `LANGSMITH_API_KEY`).

Provider debugging with delve: `go run . -debug` and follow the printed `TF_REATTACH_PROVIDERS` instructions.

Go version: 1.24+.

## Architecture

This is a [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) (Protocol 6) provider for the LangSmith REST API.

Layering:

1. **`main.go`** — provider binary entrypoint. `version` is injected via ldflags; `-debug` enables delve attach.
2. **`internal/provider/provider.go`** — registers all resources and data sources. The `Configure` method resolves credentials (precedence: explicit attribute > `LANGSMITH_API_KEY` / `LANGSMITH_API_URL` / `LANGSMITH_TENANT_ID` env vars > defaults), builds a `*client.Client`, and validates credentials by calling `/api/v1/info` before handing the client to every resource/data source via `resp.ResourceData` / `resp.DataSourceData`.
3. **`internal/provider/<name>_resource.go` / `<name>_data_source.go`** — one file per Terraform type. Each defines a `*Model` struct (Terraform state), an `apiRequest` / `apiResponse` struct (wire format), schema, and CRUD methods. Resources call into the shared `*client.Client`.
4. **`internal/client/client.go`** — the single HTTP layer. All resources go through it; do not construct `http.Request`s elsewhere. Sets `X-API-Key`, `X-Tenant-Id` (when present), and `User-Agent`. Built-in retry: up to 5 retries on 429 and 5xx, with exponential backoff + jitter; honors `Retry-After` on 429 (capped at 60s). Response body capped at 10 MB. `client.IsNotFound(err)` is the standard way to detect 404s — use it in `Read` to remove resources from state.

Conventions:

- Resource type names are `langsmith_<name>`; the Go constructor is `NewXxxResource` / `NewXxxDataSource` and must be added to the slice returned by `Resources` / `DataSources` in `provider.go`.
- A handful of resources are create+delete only because the LangSmith API returns secrets only at creation (e.g. `service_key`, `service_account`). Mark such attributes `Sensitive: true` and use `RequiresReplace` plan modifiers where update is impossible.
- For attributes that carry server-returned JSON (e.g. `extra` fields), use the helpers in `internal/provider/json_helpers.go` — `normalizeJSON` / `jsonStringValue` prevent phantom diffs from key reordering and whitespace.
- Plan preservation: several resources guard against unknown values during plan (see recent commits on `dataset` / `annotation_queue`); follow that pattern when adding computed-after-apply fields.
- `docs/` is generated — never hand-edit. Schemas + `examples/resources/langsmith_<name>/` are the source of truth; run `make generate` after changes.

## Adding a new resource

1. Create `internal/provider/<name>_resource.go` (+ optional `_data_source.go`) following the pattern in `project_resource.go` or `dataset_resource.go`.
2. Register the constructor in the `Resources` / `DataSources` slices in `provider.go`.
3. Use the shared `*client.Client` for all HTTP — never build requests directly.
4. Add `internal/provider/<name>_resource_test.go`. Unit tests run on every CI build; acceptance tests are gated by `TF_ACC=1` and `testAccPreCheck`.
5. Add an example under `examples/resources/langsmith_<name>/resource.tf` (required for docs generation).
6. Run `make generate` and commit the resulting files under `docs/`.

## Environment

- `LANGSMITH_API_KEY` — required for `make testacc` and for provider config when `api_key` isn't set.
- `LANGSMITH_TENANT_ID` — required when using an org-scoped API key.
- `LANGSMITH_API_URL` — override for self-hosted LangSmith instances (defaults to `https://api.smith.langchain.com`).
- `TF_LOG=DEBUG` — useful when running Terraform locally against the provider.
