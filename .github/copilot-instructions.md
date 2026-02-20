<!-- Copilot / AI agent instructions for contributors working on the Terraform provider -->
# Copilot instructions — terraform-provider-langsmith

Purpose: give AI coding agents the minimal, repository-specific knowledge needed to be productive.

- Big picture
  - This repository implements a Terraform provider (Go) for the LangSmith API. See `main.go` for provider startup.
  - Runtime flow: Terraform -> provider (`internal/provider/provider.go`) -> typed resource implementations in `internal/provider/*.go` -> HTTP client in `internal/client/client.go` -> LangSmith REST API.
  - Docs & examples are generated from schemas and examples under `docs/` and `examples/` via `make generate`.

- Key files & patterns to inspect first
  - `main.go`: provider binary entrypoint and versioning.
  - `internal/client/client.go`: central HTTP client, auth, and base URL handling (supports `LANGSMITH_API_URL`).
  - `internal/provider/provider.go`: registers resources and data sources.
  - `internal/provider/*_resource.go` and `*_data_source.go`: individual Terraform resources/data sources. Example: `project_resource.go` and `dataset_resource.go`.
  - `internal/provider/*_test.go`: unit tests that show intended behavior and test helpers (use these when adding features).
  - `examples/` and `docs/`: add matching example HCL and docs when adding resources.
  - `internal/provider/json_helpers.go`: repo-specific JSON handling and helpers — reuse for serialization consistency.
  - `GNUmakefile`: canonical build/test/generate commands used in CI.

- Developer workflows (explicit commands)
  - Build: `make build`
  - Unit tests: `make test`
  - Acceptance tests (live API): set `LANGSMITH_API_KEY` and `LANGSMITH_TENANT_ID`, then `make testacc`.
  - Lint: `make lint` (golangci-lint)
  - Regenerate docs/examples: `make generate` and commit changes in `docs/`.
  - Local provider testing: `make install` + add dev override to `~/.terraformrc` (see README).

- Environment & secrets
  - API key: `LANGSMITH_API_KEY` (or `api_key` provider attribute)
  - Tenant/workspace id: `LANGSMITH_TENANT_ID` (required for org-scoped keys)
  - Custom API URL: `LANGSMITH_API_URL` or `api_url` provider attribute for self-hosting

- Project-specific conventions
  - Resource names are `langsmith_*` and live as Go structs + CRUD functions in `internal/provider`.
  - Many resources are create-only for sensitive keys (e.g. `service_key`, `service_account`) — the API only returns secrets at creation.
  - New resource pattern: add `*_resource.go` + optional `*_data_source.go` in `internal/provider`, unit tests `*_resource_test.go`, an example under `examples/resources/`, then run `make generate` to produce docs under `docs/resources/`.
  - Tests: unit tests live alongside resources; acceptance tests exercise live API (do not run on CI without keys).

- Integration & CI notes
  - The provider uses Go modules (`go.mod`) and requires Go >= 1.24.
  - CI enforces generated docs are current; run `make generate` before PRs that change schemas/examples.
  - Acceptance tests mutate remote state — use a dedicated, disposable workspace and service keys.

- Quick debugging tips specific to this repo
  - To inspect provider logs when running Terraform locally, enable Terraform logging: `TF_LOG=DEBUG terraform apply`.
  - Use unit tests to replicate resource-level behavior (`go test ./internal/provider -run TestName`).

**How to add a new resource (quick walkthrough)**

- 1. Add resource file: create `internal/provider/<name>_resource.go` and implement the Terraform schema and CRUD functions (`Create`, `Read`, `Update`, `Delete`). Follow patterns in `project_resource.go` or `dataset_resource.go`.
- 2. Register resource: update `internal/provider/provider.go` to register `langsmith_<name>` in the resource map so Terraform can discover it.
- 3. Use the central client: call `internal/client/client.go` from the resource code for HTTP requests and authentication; reuse existing helpers and error handling.
- 4. Tests: add unit tests `internal/provider/<name>_resource_test.go` exercising schema and CRUD logic. Run `go test ./internal/provider -run TestYourName`.
- 5. Examples: add an example HCL under `examples/resources/langsmith_<name>/` demonstrating usage (required for docs generation).
- 6. Docs: run `make generate` to regenerate `docs/` from schemas + examples and commit the generated files; CI will fail if docs are stale.
- 7. Acceptance tests: add acceptance tests if the resource interacts with live API; run them with `make testacc` (requires `LANGSMITH_API_KEY` and `LANGSMITH_TENANT_ID`).
- 8. Conventions to follow: use `json_helpers.go` for consistent JSON serialization, keep resource functions thin (delegate HTTP to `internal/client`), and mark sensitive attributes as `Sensitive: true` when appropriate.

If anything here is unclear or you want more detail (example locations, test patterns, or how a particular resource maps to an API endpoint), tell me which area to expand and I will iterate.
