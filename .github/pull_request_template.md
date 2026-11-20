## Related Issue

Fixes # <!-- INSERT ISSUE NUMBER -->

## Description

Describe the changes made and the reasoning behind your approach.

## Testing

- [ ] Lint and unit tests pass (`make lint`, `make test`)
- [ ] Acceptance tests pass (`make testacc`), or note which were not run and why. They need `LANGSMITH_API_KEY`, plus `LANGSMITH_WORKSPACE_ID` for an org-scoped key. Fork PRs do not receive CI secrets; see [CONTRIBUTING.md](https://github.com/bogware/terraform-provider-langsmith/blob/main/CONTRIBUTING.md#fork-pull-requests-and-github-actions-secrets).
- [ ] Generated docs are up to date (`make generate`) if you changed a schema or `examples/`

## Checklist

- [ ] Code follows the project conventions in CLAUDE.md and CONTRIBUTING.md
- [ ] No breaking changes to existing resources
- [ ] `CHANGELOG.md` updated if the change is user-visible
