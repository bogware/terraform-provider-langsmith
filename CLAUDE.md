# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common commands

All workflow commands live in `GNUmakefile`:

- `make build` — compile the provider
- `make install` — install the provider binary into `GOBIN` (used together with a `~/.terraformrc` dev override; see README)
- `make test` — unit tests (`go test -v -cover -timeout=120s -parallel=10 ./...`)
- `make testacc` — acceptance tests against the live LangSmith API; requires `LANGSMITH_API_KEY` and (for org-scoped keys) `LANGSMITH_WORKSPACE_ID`. Sets `TF_ACC=1`. These create and delete real resources — use a dedicated, disposable workspace.
- `make lint` — `golangci-lint run`
- `make generate` — regenerates `docs/` and re-formats `examples/`. Runs `cd tools; go generate ./...` — **`tools/` is a separate Go module** (its own `go.mod`, `go 1.24.0`; the codegen tools `copywrite` + `terraform-plugin-docs` are pinned there, not in the root module). The directives run, in order: copywrite headers → `terraform fmt -recursive ../examples/` → `tfplugindocs generate`, and a real `terraform` binary must be on `PATH`. CI runs `make generate` then `git diff --exit-code`, so any stale `docs/` (or unformatted example) fails the build — run it after any schema or example change and commit the result.

Single test: `go test ./internal/provider -run TestName -v`. Acceptance variants are gated by `TF_ACC=1` and `testAccPreCheck` (which requires `LANGSMITH_API_KEY`).

**Avoid bare `make` and `make fmt`.** The default target is `fmt lint install generate`, and `fmt` is `gofmt -s -w -e .` across the whole repo — it rewrites hundreds of untouched files into an unreviewable diff. Nothing in CI checks gofmt (the `.golangci.yml` `formatters:` block enables none), so format only the files you actually edit.

Provider debugging with delve: `go run . -debug` and follow the printed `TF_REATTACH_PROVIDERS` instructions.

Go version: root `go.mod` declares `go 1.25.8` and pins `toolchain go1.26.5`. CI **and goreleaser** both derive their Go version from `go.mod` via `go-version-file`, so the `toolchain` line is what governs the standard library compiled into released binaries — bump it when a Go security release lands (`govulncheck ./...` must stay clean).

## Architecture

This is a [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) (Protocol 6) provider for the LangSmith REST API.

Layering:

1. **`main.go`** — provider binary entrypoint. `version` is injected via ldflags; `-debug` enables delve attach.
2. **`internal/provider/provider.go`** — registers all resources and data sources, and resolves configuration. The provider schema has exactly four attributes: `api_key`, `api_url`, `workspace_id`, `self_hosted`. Precedence is explicit attribute > env var (`LANGSMITH_API_KEY` / `LANGSMITH_API_URL` / `LANGSMITH_WORKSPACE_ID` / `LANGSMITH_SELF_HOSTED`) > default; a non-null attribute overrides its env var, and `api_key` has no default (empty raises "Missing API Key"). `checkUnknownConfig` **rejects any provider attribute that is unknown at plan time** — a provider is configured before the resources it depends on are applied, so `workspace_id = langsmith_workspace.prod.id` in a `provider` block would otherwise silently degrade to `""` and mis-scope every call. An `http://` `api_url` produces a warning (not an error) because self-hosted instances on trusted networks legitimately use it. Configure then validates credentials with `GET /api/v1/info` before handing the `*client.Client` to every resource/data source via `resp.ResourceData` / `resp.DataSourceData`. **There is no provider-level `tenant_id` attribute** — it was removed provider-wide in 1.0; don't reintroduce one.
3. **`internal/provider/<name>_resource.go` / `<name>_data_source.go`** — one file per Terraform type. Each defines a `*Model` struct (Terraform state), an `apiRequest` / `apiResponse` struct (wire format), schema, and CRUD methods. Resources call into the shared `*client.Client`.
4. **`internal/client/client.go`** — the single HTTP layer (~316 lines; `http.Client` timeout 120s). All resources go through it; never construct `http.Request`s elsewhere. Exposes `Get` / `Post` / `Patch` / `Put` / `Delete`, plus `PostWithQuery` / `PutWithQuery` / `DeleteWithQuery` and `DeleteWithBody`.

### Client invariants (`internal/client/client.go`)

- **Auth headers.** `X-API-Key` only (no `Authorization` header); `Accept: application/json` is always set, `X-Tenant-Id` and `User-Agent` only when non-empty, and `Content-Type: application/json` only when there is a request body.
- **Retries.** Built-in retry (`MaxRetries=5`, i.e. up to 6 attempts) on **408, 429, all 5xx, and transport/body-read errors**, with exponential backoff + ±20% jitter. `Retry-After` is honored only on 429 (parsed as integer seconds, capped at 60s — a 429 without a usable `Retry-After` waits a fixed 5s, and both paths skip the next exponential wait to avoid double-backoff).
- **Errors.** Non-2xx returns a typed `*APIError{Method, Path, StatusCode, Body}`, preserved through the retry wrapper so `errors.As` still works; `client.IsNotFound(err)` checks `StatusCode == 404` — use it in `Read` to remove resources from state.
- **Body cap.** The response body is `io.LimitReader`-capped at 10 MB and **silently truncated** (no error), which can surface later as a confusing JSON unmarshal failure.
- **Two security guards worth not breaking.** `dropAuthOnCrossHostRedirect` follows redirects but strips `X-API-Key` / `X-Tenant-Id` whenever the host changes (and caps at 10 redirects), so a redirect can't leak credentials to another host. `doRequest` rejects any path containing `?` or `#` — query params always arrive via the separate `query` argument, so those characters can only come from an unescaped caller-supplied segment (repo handle, tag name, secret key), where a `#` would truncate the URL and delete the parent object instead of the child.

### API path families and self-hosted

LangSmith serves **two path families** and they are not interchangeable:

- Legacy endpoints: `/api/v1/...`
- Newer "platform" endpoints: `/v1/platform/...` — note the **absence** of `/api` on Cloud.

Not every endpoint follows the obvious pattern and the published OpenAPI spec is incomplete, so probe a live instance before "fixing" a path that looks wrong.

`Client.resolvePath` implements self-hosted support: when `SelfHosted` is true, and **only** for paths starting `/v1/platform/`, it rewrites to `/api/v1/platform/...`. Self-hosted shares one host between frontend and API, mounting the API under `/api`; an unmatched path falls through to the SPA (200 with HTML, or 405 to a POST) rather than erroring usefully. The rewrite is deliberately *not* a blanket prefix — agent builder, sandboxes, and other fleet families are served at their own root on self-hosted and would break. `SelfHosted` is a plain struct field, so `WithWorkspaceID`'s shallow copy inherits it (there is a test asserting this).

## Conventions

- Resource type names are `langsmith_<name>`; the Go constructor is `NewXxxResource` / `NewXxxDataSource` and must be added to the slice returned by `Resources` / `DataSources` in `provider.go` (currently 62 resources, 63 data sources — the two slices are *not* the same length). The convention is `New<Camel>Resource` → `langsmith_<snake>` TypeName → `<snake>_resource.go` (acronym casing like SCIM/SSO/MCP/TTL is intentional). **One exception breaks the 1:1 file/name pairing:** `NewMCPVendorSettingsResource` (`langsmith_mcp_vendor_settings`) vs. `NewMCPVendorDataSource` (`langsmith_mcp_vendor`) — same domain, different base name.
- `Configure` on every resource/data source is identical boilerplate that type-asserts `req.ProviderData` to `*client.Client`.
- **`ImportState` is NOT universally a passthrough.** `resource.ImportStatePassthroughID` is correct *only* when `Read` can fetch the object from that single ID alone. If `Read` needs a parent ID or a natural key (`dataset_id`, `session_id`, `queue_id`, repo `owner`/`handle`, `vendor_slug`, secret `key`, …), a passthrough silently produces an unresolvable object and the import fails or yields wrong state — this was a real bug class fixed in 1.0. 22 of the 62 resources parse a **composite** ID instead (`<parent>/<child>` or `<parent>:<child>`; see `secret`, `tagging`, `comparative_experiment`, `alert_rule`, `experiment_view_override`). When adding a resource, ask what `Read` requires and pick accordingly. Import ID formats are listed in `INSTALL.md`.
- `golangci-lint` (config in `.golangci.yml`, **v2 schema**) **bans all `terraform-plugin-sdk/v2` imports via depguard** — use `terraform-plugin-framework` for resources and `terraform-plugin-testing` for tests only. Lint runs inside the CI `build` job, not as a separate job, pinned to **golangci-lint v2.12.2**: v1 is EOL and its last build refuses to lint this module because of the `go.mod` toolchain pin, so a local v1 install can report clean while CI fails. Enabled beyond defaults: `copyloopvar`, `depguard`, `durationcheck`, `forcetypeassert`, `godot`, `makezero`, `misspell`, `nilerr`, `predeclared`, `unconvert`, `unparam` (`godot` means comments must end in a period).
- A handful of resources are create+delete only because the LangSmith API returns secrets only at creation (e.g. `service_key`, `service_account`). Mark such attributes `Sensitive: true` and use `RequiresReplace` plan modifiers where update is impossible. Concrete patterns to copy: `service_key`'s `Read` lists `/api/v1/orgs/current/service-keys` and linear-searches by ID (no single-GET endpoint), keeping the original secret via `Computed + Sensitive + UseStateForUnknown`. When a mutation endpoint returns only a status message (e.g. `annotation_queue` `Update` PATCHes then GETs to repopulate state), don't depend on the mutation's response body for state.
- For attributes that carry server-returned JSON (e.g. `extra` fields), use the helpers in `internal/provider/json_helpers.go` — `normalizeJSON` / `jsonStringValue` prevent phantom diffs from key reordering and whitespace; `stripJSONKey` drops a server-injected key while preserving the saved value, and `jsonEmptyArrayIsNull` maps `[]` to null.
- Plan preservation: several resources guard against unknown values during plan (see recent commits on `dataset` / `annotation_queue`); follow that pattern when adding computed-after-apply fields.
- Per-resource workspace override: most resources expose an `Optional + Computed` `workspace_id` attribute (with `UseStateForUnknown`) that overrides the provider-level workspace for that resource's API calls. Use the helpers in `internal/provider/workspace_helpers.go`: `effectiveClient(r.client, data.WorkspaceID)` at the top of each CRUD method (it wraps `client.WithWorkspaceID`, a shallow copy that swaps the `X-Tenant-Id` header), and `finalizeWorkspaceID(&data.WorkspaceID, c, apiValue, &resp.Diagnostics)` in **every** code path that sets state — it guarantees `workspace_id` is never left unknown after apply (Terraform hard-fails otherwise) and warns, keeping the user's value, when config and API disagree. LangSmith APIs inconsistently return the workspace as `workspace_id` or `tenant_id`: decode **both** keys in wire structs and pass `firstNonEmpty(api.WorkspaceID, api.TenantID)` as apiValue (empty string if the endpoint returns neither). **Never add a Terraform-facing `tenant_id` attribute.** This applies only to the schema: wire structs must still decode `json:"tenant_id"`, because many endpoints return only that key.
- `workspace_id` must carry **both** `UseStateForUnknown()` **and** `RequiresReplace()`. It selects which workspace the call targets, and objects never move between workspaces, so a change must destroy and recreate. It is also the attribute most likely to be left unknown after apply — hence `finalizeWorkspaceID` in every state-setting path.
- Because the provider rejects plan-time-unknown provider config, per-resource `workspace_id` — not a provider alias — is the supported way to manage a workspace created in the same apply. README documents this; keep new resources consistent with it.
- `docs/` is generated — never hand-edit. Schemas + `examples/resources/langsmith_<name>/` are the source of truth; run `make generate` after changes.

## Testing conventions

- `internal/provider/provider_test.go` holds the two shared helpers: `testAccProtoV6ProviderFactories` (Protocol 6 factory map) and `testAccPreCheck` (fails when `LANGSMITH_API_KEY` is unset).
- Beyond `TF_ACC`, ~41 distinct `LANGSMITH_TEST_*` env vars gate individual acceptance tests that need an entitlement, elevated permissions, or a pre-existing object — e.g. `LANGSMITH_TEST_ABAC`, `LANGSMITH_TEST_GATEWAY_ENABLED`, `LANGSMITH_TEST_RUN_ID`, `LANGSMITH_TEST_S3_BUCKET`. The convention is `t.Skip("Set LANGSMITH_TEST_X=1 to enable (...why...)")`. Follow it for anything that would otherwise fail on a standard workspace or mutate shared org state; a few tests are unconditionally skipped where the tier simply cannot be tested.
- Unit tests (schema/helper level) must pass with no credentials at all — CI's `unit-tests` job runs `./...` without `TF_ACC`.

## CI & release

- **CI** (`.github/workflows/test.yml`, all jobs on `ubuntu-latest`, no OS/Go matrix, all actions SHA-pinned): `build` (lint runs *inside* this job), `generate` (runs `make generate` then `git diff --exit-code` — the stale-docs gate), `unit-tests`, and `acceptance-tests`. Acceptance tests **run in CI on every push/PR**, scoped to `./internal/provider/` with `-timeout 30m` and using repo secrets `LANGSMITH_API_KEY` + `LANGSMITH_WORKSPACE_ID` — unlike `make testacc`, which is `./...` at `-timeout 120m`. Doc-only changes (`README.md` / `CHANGELOG.md` / `CLAUDE.md`) are skipped via `paths-ignore`.
- **Release** (`.github/workflows/release.yml`) is triggered only by pushing a `v*` git tag: a goreleaser job imports a GPG key (secrets `GPG_PRIVATE_KEY` + `PASSPHRASE`), builds `CGO_ENABLED=0 -trimpath` with `main.version` / `main.commit` injected, and signs the `SHA256SUMS` file. `terraform-registry-manifest.json` declares Terraform protocol `6.0`.
- **Contributor conventions** (there is no `CONTRIBUTING.md`): extra rules live in `.github/copilot-instructions.md`; the PR template wants a linked issue (`Fixes #…`) plus `make test` / `make testacc` / `make generate` checkboxes. `CHANGELOG.md` uses a top `## X.Y.Z (Unreleased)` heading with entries grouped under uppercase category headers (BUG FIXES, FEATURES, ENHANCEMENTS, BREAKING CHANGES, …) referencing PR/issue numbers.

## Adding a new resource

1. Create `internal/provider/<name>_resource.go` (+ optional `_data_source.go`) following the pattern in `project_resource.go` or `dataset_resource.go`.
2. Register the constructor in the `Resources` / `DataSources` slices in `provider.go`.
3. Use the shared `*client.Client` for all HTTP — never build requests directly.
4. Add `internal/provider/<name>_resource_test.go`. Unit tests run on every CI build; acceptance tests are gated by `TF_ACC=1` and `testAccPreCheck` (plus a `LANGSMITH_TEST_*` gate if the test needs an entitlement or pre-existing object).
5. Add an example under `examples/resources/langsmith_<name>/resource.tf` (required for docs generation).
6. Run `make generate` and commit the resulting files under `docs/`.

## Environment

- `LANGSMITH_API_KEY` — required for `make testacc` and for provider config when `api_key` isn't set.
- `LANGSMITH_WORKSPACE_ID` — required when using an org-scoped API key.
- `LANGSMITH_API_URL` — override for self-hosted LangSmith instances (defaults to `https://api.smith.langchain.com`).
- `LANGSMITH_SELF_HOSTED` — `1` / `true` / `yes` enables the self-hosted path rewrite; equivalent to `self_hosted = true`.
- `LANGSMITH_TEST_*` — per-test acceptance gates (see Testing conventions).
- `TF_LOG=DEBUG` — useful when running Terraform locally against the provider.
