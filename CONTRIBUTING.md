# Contributing

Thanks for helping improve this provider. Please read the [Code of Conduct](.github/CODE_OF_CONDUCT.md).

## Before you open a pull request

From the repository root (GNU Make reads `GNUmakefile` as `make`):

```bash
make lint    # golangci-lint — install: https://golangci-lint.run/welcome/install/
make test    # unit tests (no LangSmith account required)
```

If you changed resource schemas or anything under `examples/`, regenerate docs and commit the `docs/` updates:

```bash
make generate
```

CI runs the same checks (see `.github/workflows/test.yml`).

## Acceptance tests (`TF_ACC`)

Acceptance tests call the real LangSmith API. They run only when Terraform’s acceptance mode is enabled by setting **`TF_ACC=1`** (the `make testacc` target does this for you).

Set credentials in your environment (same names CI uses):

| Variable | When you need it |
|----------|------------------|
| `LANGSMITH_API_KEY` | Always for acceptance tests |
| `LANGSMITH_TENANT_ID` | Org-scoped keys (same as provider docs) |
| `LANGSMITH_API_URL` | Optional; self-hosted API base URL |

```bash
export LANGSMITH_API_KEY="lsv2_..."
export LANGSMITH_TENANT_ID="your-workspace-uuid"   # if using an org-scoped key
make testacc
```

Some tests are skipped when the environment or plan tier does not support them; that is expected.

## Fork pull requests and GitHub Actions secrets

Workflows read **`LANGSMITH_API_KEY`** and **`LANGSMITH_TENANT_ID`** from [encrypted repository secrets](https://docs.github.com/en/actions/security-guides/using-secrets-in-github-actions). Those secrets are **not** passed to workflows triggered from forks, so the **acceptance** job on a fork PR typically cannot authenticate.

What we still expect from fork contributors:

- `make lint` and `make test` pass locally.
- `make generate` run and committed when schemas/examples change.
- If you have credentials, run `make testacc` locally and mention the result in the PR description.

Never commit API keys or tenant IDs; use secrets locally and in GitHub only.

## Releases

Maintainers cut releases by pushing a version tag (`v*`), which runs `.github/workflows/release.yml` (GoReleaser). Contributors do not need release tooling for normal PRs.
