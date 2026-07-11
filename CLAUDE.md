# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common commands

All workflow commands live in `GNUmakefile`:

- `make build` — compile the provider
- `make install` — install the provider binary into `GOBIN` (used together with a `~/.terraformrc` dev override; see README)
- `make test` — unit tests (`go test -v -cover -timeout=120s -parallel=10 ./...`)
- `make testacc` — acceptance tests against the live LangSmith API; requires `LANGSMITH_API_KEY` and (for org-scoped keys) `LANGSMITH_WORKSPACE_ID`. Sets `TF_ACC=1`. These create and delete real resources — use a dedicated, disposable workspace.
- `make lint` — `golangci-lint run`
- `make fmt` — `gofmt -s -w -e .`
- `make generate` — regenerates `docs/` and re-formats `examples/`. Runs `cd tools; go generate ./...` — **`tools/` is a separate Go module** (its own `go.mod`; the codegen tools `copywrite` + `terraform-plugin-docs` are pinned there, not in the root module). The directives run, in order: copywrite headers → `terraform fmt -recursive ../examples/` → `tfplugindocs generate`, and a real `terraform` binary must be on `PATH`. CI runs `make generate` then `git diff --exit-code`, so any stale `docs/` (or unformatted example) fails the build — run it after any schema or example change and commit the result.

Single test: `go test ./internal/provider -run TestName -v`. Acceptance variants are gated by `TF_ACC=1` and `testAccPreCheck` (which requires `LANGSMITH_API_KEY`).

Provider debugging with delve: `go run . -debug` and follow the printed `TF_REATTACH_PROVIDERS` instructions.

Go version: root `go.mod` declares `go 1.25.0` and pins `toolchain go1.26.5`. CI **and goreleaser** both derive their Go version from `go.mod` via `go-version-file`, so the `toolchain` line is what governs the standard library compiled into released binaries — bump it when a Go security release lands (`govulncheck ./...` must stay clean).

## Architecture

This is a [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) (Protocol 6) provider for the LangSmith REST API.

Layering:

1. **`main.go`** — provider binary entrypoint. `version` is injected via ldflags; `-debug` enables delve attach.
2. **`internal/provider/provider.go`** — registers all resources and data sources. The `Configure` method resolves credentials (precedence: explicit attribute > `LANGSMITH_API_KEY` / `LANGSMITH_API_URL` / `LANGSMITH_WORKSPACE_ID` env vars > defaults; a non-null attribute overrides its env var, and `api_key` has no default — empty raises a "Missing API Key" error), builds a `*client.Client`, and validates credentials by calling `/api/v1/info` before handing the client to every resource/data source via `resp.ResourceData` / `resp.DataSourceData`. The deprecated provider-level `tenant_id` / `LANGSMITH_TENANT_ID` backfills `workspace_id` only when it's empty and raises a **hard error if both are set to different values**.
3. **`internal/provider/<name>_resource.go` / `<name>_data_source.go`** — one file per Terraform type. Each defines a `*Model` struct (Terraform state), an `apiRequest` / `apiResponse` struct (wire format), schema, and CRUD methods. Resources call into the shared `*client.Client`.
4. **`internal/client/client.go`** — the single HTTP layer (`http.Client` timeout 120s). All resources go through it; never construct `http.Request`s elsewhere. Exposes `Get` / `Post` / `Patch` / `Put` / `Delete`, plus `PostWithQuery` / `PutWithQuery` / `DeleteWithQuery` and `DeleteWithBody`. Auth is `X-API-Key` only (no `Authorization` header); `Accept: application/json` is always set, `X-Tenant-Id` and `User-Agent` only when non-empty, and `Content-Type: application/json` only when there is a request body. Built-in retry (`MaxRetries=5`, i.e. up to 6 attempts) on **408, 429, all 5xx, and transport/body-read errors**, with exponential backoff + ±20% jitter; `Retry-After` is honored only on 429 (parsed as integer seconds, capped at 60s — a 429 without a usable `Retry-After` waits a fixed 5s). Non-2xx returns a typed `*APIError{Method, Path, StatusCode, Body}`, preserved through the retry wrapper so `errors.As` still works; `client.IsNotFound(err)` checks `StatusCode == 404` — use it in `Read` to remove resources from state. The response body is `io.LimitReader`-capped at 10 MB and **silently truncated** (no error), which can surface later as a confusing JSON unmarshal failure.

Conventions:

- Resource type names are `langsmith_<name>`; the Go constructor is `NewXxxResource` / `NewXxxDataSource` and must be added to the slice returned by `Resources` / `DataSources` in `provider.go` (currently 62 resources, 63 data sources — the two slices are *not* the same length). The convention is `New<Camel>Resource` → `langsmith_<snake>` TypeName → `<snake>_resource.go` (acronym casing like SCIM/SSO/MCP/TTL is intentional). **One exception breaks the 1:1 file/name pairing:** `NewMCPVendorSettingsResource` (`langsmith_mcp_vendor_settings`) vs. `NewMCPVendorDataSource` (`langsmith_mcp_vendor`) — same domain, different base name.
- `Configure` on every resource/data source is identical boilerplate that type-asserts `req.ProviderData` to `*client.Client`.
- **`ImportState` is NOT universally a passthrough.** `resource.ImportStatePassthroughID` is correct *only* when `Read` can fetch the object from that single ID alone. If `Read` needs a parent ID or a natural key (`dataset_id`, `session_id`, `queue_id`, repo `owner`/`handle`, `vendor_slug`, secret `key`, …), a passthrough silently produces an unresolvable object and the import fails or yields wrong state — this was a real bug class fixed in 1.0. Those resources parse a **composite** ID instead (`<parent>/<child>` or `<parent>:<child>`; see `secret`, `tagging`, `comparative_experiment`, `alert_rule`, `experiment_view_override`). When adding a resource, ask what `Read` requires and pick accordingly. Import ID formats are listed in `INSTALL.md`.
- `golangci-lint` (config in `.golangci.yml`) **bans all `terraform-plugin-sdk/v2` imports via depguard** — use `terraform-plugin-framework` for resources and `terraform-plugin-testing` for tests only. Lint runs inside the CI `build` job, not as a separate job.
- A handful of resources are create+delete only because the LangSmith API returns secrets only at creation (e.g. `service_key`, `service_account`). Mark such attributes `Sensitive: true` and use `RequiresReplace` plan modifiers where update is impossible. Concrete patterns to copy: `service_key`'s `Read` lists `/api/v1/orgs/current/service-keys` and linear-searches by ID (no single-GET endpoint), keeping the original secret via `Computed + Sensitive + UseStateForUnknown`. When a mutation endpoint returns only a status message (e.g. `annotation_queue` `Update` PATCHes then GETs to repopulate state), don't depend on the mutation's response body for state.
- For attributes that carry server-returned JSON (e.g. `extra` fields), use the helpers in `internal/provider/json_helpers.go` — `normalizeJSON` / `jsonStringValue` prevent phantom diffs from key reordering and whitespace.
- Plan preservation: several resources guard against unknown values during plan (see recent commits on `dataset` / `annotation_queue`); follow that pattern when adding computed-after-apply fields.
- Per-resource workspace override: most resources expose an `Optional + Computed` `workspace_id` attribute (with `UseStateForUnknown`) that overrides the provider-level workspace for that resource's API calls. Use the helpers in `internal/provider/workspace_helpers.go`: `effectiveClient(r.client, data.WorkspaceID)` at the top of each CRUD method (it wraps `client.WithWorkspaceID`, a shallow copy that swaps the `X-Tenant-Id` header), and `finalizeWorkspaceID(&data.WorkspaceID, c, apiValue, &resp.Diagnostics)` in **every** code path that sets state — it guarantees `workspace_id` is never left unknown after apply (Terraform hard-fails otherwise). LangSmith APIs inconsistently return the workspace as `workspace_id` or `tenant_id`: decode **both** keys in wire structs and pass `firstNonEmpty(api.WorkspaceID, api.TenantID)` as apiValue (empty string if the endpoint returns neither). **Never add a Terraform-facing `tenant_id` attribute** — it was removed provider-wide in 1.0. This applies only to the schema: wire structs must still decode `json:"tenant_id"`, because many endpoints return only that key.
- `workspace_id` must carry **both** `UseStateForUnknown()` **and** `RequiresReplace()`. It selects which workspace the call targets, and objects never move between workspaces, so a change must destroy and recreate. It is also the attribute most likely to be left unknown after apply — hence `finalizeWorkspaceID` in every state-setting path.
- `docs/` is generated — never hand-edit. Schemas + `examples/resources/langsmith_<name>/` are the source of truth; run `make generate` after changes.

## Adding a new resource

1. Create `internal/provider/<name>_resource.go` (+ optional `_data_source.go`) following the pattern in `project_resource.go` or `dataset_resource.go`.
2. Register the constructor in the `Resources` / `DataSources` slices in `provider.go`.
3. Use the shared `*client.Client` for all HTTP — never build requests directly.
4. Add `internal/provider/<name>_resource_test.go`. Unit tests run on every CI build; acceptance tests are gated by `TF_ACC=1` and `testAccPreCheck`.
5. Add an example under `examples/resources/langsmith_<name>/resource.tf` (required for docs generation).
6. Run `make generate` and commit the resulting files under `docs/`.

## CI & release

- **CI** (`.github/workflows/test.yml`, all jobs on `ubuntu-latest`, no OS/Go matrix): `build` (lint runs *inside* this job), `generate` (runs `make generate` then `git diff --exit-code` — the stale-docs gate), `unit-tests`, and `acceptance-tests`. Acceptance tests **run in CI on every push/PR**, scoped to `./internal/provider/` with `-timeout 30m` and using repo secrets `LANGSMITH_API_KEY` + `LANGSMITH_WORKSPACE_ID` — unlike `make testacc`, which is `./...` at `-timeout 120m`. Doc-only changes (`README.md` / `CHANGELOG.md` / `CLAUDE.md`) are skipped via `paths-ignore`.
- **Release** (`.github/workflows/release.yml`) is triggered only by pushing a `v*` git tag: a goreleaser job imports a GPG key (secrets `GPG_PRIVATE_KEY` + `PASSPHRASE`), builds `CGO_ENABLED=0 -trimpath` with `main.version` / `main.commit` injected, and signs the `SHA256SUMS` file. `terraform-registry-manifest.json` declares Terraform protocol `6.0`.
- **Contributor conventions** (there is no `CONTRIBUTING.md`): extra rules live in `.github/copilot-instructions.md`; the PR template wants a linked issue (`Fixes #…`) plus `make test` / `make testacc` / `make generate` checkboxes. `CHANGELOG.md` uses a top `## X.Y.Z (Unreleased)` heading with entries grouped under uppercase category headers (BUG FIXES, FEATURES, ENHANCEMENTS, BREAKING CHANGES, …) referencing PR/issue numbers.

## Environment

- `LANGSMITH_API_KEY` — required for `make testacc` and for provider config when `api_key` isn't set.
- `LANGSMITH_WORKSPACE_ID` — required when using an org-scoped API key.
- `LANGSMITH_API_URL` — override for self-hosted LangSmith instances (defaults to `https://api.smith.langchain.com`).
- `TF_LOG=DEBUG` — useful when running Terraform locally against the provider.
