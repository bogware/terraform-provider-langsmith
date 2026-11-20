# Contributing

This project follows the [Code of Conduct](.github/CODE_OF_CONDUCT.md).

For toolchain versions, everyday `make` targets and the local dev override, see **Development** in [README.md](README.md). [CLAUDE.md](CLAUDE.md) holds the architecture notes and codebase conventions: how resources are laid out, import ID formats, per-resource `workspace_id`, and the JSON helpers. This file covers what CI expects and how acceptance tests and secrets behave.

## Before opening a pull request

From the repository root:

```bash
make lint    # golangci-lint v2 (CI pins v2.12.2; a v1 install will not lint this module)
make test    # unit tests; no LangSmith account required
```

If you change a resource schema or anything under `examples/`, regenerate the docs and commit the result under `docs/`:

```bash
make generate   # needs a terraform binary on PATH
```

Avoid bare `make` and `make fmt`. They run `gofmt` across the whole repository and rewrite many files you did not touch. Format only the files you edit.

If your change is user-visible, add an entry to `CHANGELOG.md` under the top `## X.Y.Z (Unreleased)` heading, in the matching category (BUG FIXES, ENHANCEMENTS, FEATURES, ...), and link the issue or PR.

## What CI runs

Every push and pull request runs `.github/workflows/test.yml`:

| Job | What it checks |
| --- | -------------- |
| Build | `go build` and `golangci-lint` |
| govulncheck | No known vulnerability reachable from the code. Fix a finding by bumping the dependency, or the `toolchain` line in `go.mod` for a standard-library finding. |
| Verify Generated Files | `make generate` leaves no diff, so `docs/` and `examples/` are current |
| Unit Tests | `go test ./...` with coverage printed in the log |
| Unit Tests (race) | `go test -race ./...` |
| Acceptance Tests | `./internal/provider/` against the live API, using repository secrets |

Changes that touch only `README.md`, `CHANGELOG.md` or `CLAUDE.md` skip the workflow.

## Acceptance tests (`TF_ACC`)

Acceptance tests call the live LangSmith API and create and delete real objects, so point them at a dedicated, disposable workspace. Terraform runs them only when `TF_ACC=1`; `make testacc` sets that for you.

| Variable | Notes |
| -------- | ----- |
| `LANGSMITH_API_KEY` | Required |
| `LANGSMITH_WORKSPACE_ID` | Required with an org-scoped key; without it most calls return 403 |
| `LANGSMITH_API_URL` | Optional; base URL of a self-hosted instance |
| `LANGSMITH_SELF_HOSTED` | Optional; `1` / `true` enables the self-hosted path rewrite |

```bash
export LANGSMITH_API_KEY="lsv2_..."
export LANGSMITH_WORKSPACE_ID="your-workspace-uuid"   # if using an org-scoped key
make testacc
```

Some tests need an entitlement, elevated permissions or a pre-existing object, and skip unless a `LANGSMITH_TEST_*` variable enables them. The skip message names the variable and says why. Skips are expected on a standard workspace.

Several tests run against an in-process stub of the API instead (see `promptStub` in `internal/provider/prompt_resource_test.go`). They still need `TF_ACC=1` but no credentials.

## Fork pull requests and GitHub Actions secrets

The acceptance job reads `LANGSMITH_API_KEY` and `LANGSMITH_WORKSPACE_ID` from [repository secrets](https://docs.github.com/en/actions/security-guides/using-secrets-in-github-actions). GitHub does not pass secrets to workflows triggered from a fork, so on a fork PR that job fails: `testAccPreCheck` stops any acceptance test that has no `LANGSMITH_API_KEY`.

Please still run `make lint`, `make test` and, when applicable, `make generate`, and run `make testacc` locally if you can. Say in the PR which acceptance tests you ran, or that you could not, so reviewers know what was verified.

Never commit API keys, workspace IDs or other secrets.

## Releases

Releases are cut from `v*` tags by `.github/workflows/release.yml` (GoReleaser, GPG-signed checksums). Day-to-day contributions do not need any release tooling.
