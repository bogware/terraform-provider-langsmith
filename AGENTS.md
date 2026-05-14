# AGENTS.md

## Cursor Cloud specific instructions

This is a **Go-based Terraform provider** for the LangSmith API. There is no web UI, no Node.js, and no Docker needed.

### Quick reference

| Command         | What it does                                             |
|-----------------|----------------------------------------------------------|
| `make build`    | Compile the provider                                     |
| `make test`     | Run unit tests (no API key needed)                       |
| `make testacc`  | Run acceptance tests (requires `LANGSMITH_API_KEY` + `LANGSMITH_TENANT_ID`) |
| `make lint`     | Run golangci-lint                                        |
| `make generate` | Regenerate docs from schemas + examples (requires `terraform` CLI) |
| `make install`  | Build and install provider binary to `$GOBIN`            |

See `GNUmakefile` for the full list of targets and the README for more details.

### Gotchas

- **golangci-lint version**: The `.golangci.yml` config uses v1 format. Install golangci-lint **v1.x** (e.g. v1.64.8), not v2.x. The v2 CLI requires a `version` field in the config that this repo does not include.
- **Acceptance tests hit a live API**: `make testacc` creates/modifies/deletes real LangSmith resources. You need `LANGSMITH_API_KEY` and `LANGSMITH_TENANT_ID` set. Use a dedicated disposable workspace.
- **`TestAccTenantsDataSource_basic` and `/api/v1/tenants`**: CI and many org API keys call `GET /api/v1/workspaces` successfully but get **403 Forbidden** on `GET /api/v1/tenants` (workspace-scoped credentials). The test PreCheck probes the tenants endpoint and **skips** when it sees 403 so the suite stays green; use an API key that is allowed to list tenants if you need this test to run locally or in CI.
- **Unit tests work without LangSmith secrets**: `make test` skips live acceptance tests when `TF_ACC` is unset and credentials are absent. You still need the **Terraform CLI** on `PATH`: `terraform-plugin-testing` runs `terraform` for config-driven tests. If you see `unable to verify checksums signature: openpgp: key expired`, install Terraform from [HashiCorp](https://developer.hashicorp.com/terraform/install) or point **`TF_ACC_TERRAFORM_PATH`** at your `terraform` binary so the test helper does not try to download releases via an embedded installer.
- **Doc generation**: `make generate` requires `terraform` CLI on `PATH`. After modifying any resource schema or example HCL, run `make generate` and commit the resulting `docs/` changes; CI will fail if generated docs are stale.
- **Local provider testing**: To test against real Terraform configs, run `make install`, add a dev override to `~/.terraformrc` pointing `bogware/langsmith` at your `$GOBIN`, then use `terraform plan/apply` directly (skip `terraform init`).
