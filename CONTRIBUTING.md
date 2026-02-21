# Contributing to Evidra

Thanks for contributing.

## Prerequisites

- Go 1.22+ (recommended 1.23)
- `git`

## Development Workflow

1. Fork and create a feature branch.
2. Make focused changes with tests.
3. Run local checks:

```bash
make fmt
make test
make lint
```

4. Update docs/changelog when behavior changes.
5. Open a pull request.

## Coding Guidelines

- Keep changes minimal and deterministic.
- Preserve existing behavior unless the PR explicitly changes it.
- Avoid introducing heavy dependencies.
- Keep core (`/core`) narrative-neutral.
- Put bundle-specific logic under `/bundles/*`.

## Tests

Use:

```bash
go test ./...
```

Add or update tests for all behavior changes.

## Pull Requests

PRs should include:

- Problem statement
- Scope and non-goals
- Files changed
- Validation steps/results
- Follow-up TODOs (if any)

Small, focused PRs are preferred over large refactors.
