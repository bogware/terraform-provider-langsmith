# Contributing

This project follows the [Code of Conduct](.github/CODE_OF_CONDUCT.md).

For toolchain versions and everyday commands, see **Development** in [README.md](README.md). The sections below spell out what CI expects and how acceptance tests and secrets behave.

## Before opening a pull request

From the repository root:

```bash
make lint    # golangci-lint (install: https://golangci-lint.run/welcome/install/)
make test    # unit tests; no LangSmith account required
```

If you change resource schemas or files under `examples/`, regenerate docs and commit the updates under `docs/`:

```bash
make generate
```

CI mirrors this flow (see `.github/workflows/test.yml`).

## Acceptance tests (`TF_ACC`)

Acceptance tests call the live LangSmith API. Terraform runs them only when **`TF_ACC=1`**; `make testacc` sets that for you.

Use the same environment variable names as CI and the provider:

| Variable | Notes |
| -------- | ----- |
| `LANGSMITH_API_KEY` | Required for acceptance tests |
| `LANGSMITH_TENANT_ID` | Required with org-scoped keys (same as provider docs) |
| `LANGSMITH_API_URL` | Optional; self-hosted API base URL |

Example:

```bash
export LANGSMITH_API_KEY="lsv2_..."
export LANGSMITH_TENANT_ID="your-workspace-uuid"   # if using an org-scoped key
make testacc
```

Some tests skip when the account or plan does not meet their requirements; that is expected.

## Fork pull requests and GitHub Actions secrets

Workflows load **`LANGSMITH_API_KEY`** and **`LANGSMITH_TENANT_ID`** from [repository secrets](https://docs.github.com/en/actions/security-guides/using-secrets-in-github-actions). Workflows triggered from a fork do not receive those secrets, so the acceptance job often cannot run there.

Please still run `make lint`, `make test`, and `make generate` when applicable, and run `make testacc` locally if you can. Note in the PR when acceptance tests were not run so reviewers know what was verified.

Do not commit API keys, tenant IDs, or other secrets.

## Releases

Releases are created from `v*` tags via `.github/workflows/release.yml` (GoReleaser). Day-to-day contributions do not require release tooling.
