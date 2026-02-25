# Contributing to Evidra

Thanks for contributing.

## Prerequisites

- Go 1.24+ (exact version pinned in `go.mod` via `toolchain` directive)
- `git`

## Go Toolchain Policy

`go.mod` is the single source of truth for the Go version:

```
go 1.24.6
```

Since Go 1.21, `go 1.24.6` in `go.mod` means both "minimum language version" and "required toolchain". CI (`setup-go` with `go-version-file: go.mod`) and GoReleaser read this automatically.

| File | How it gets the version |
|---|---|
| `go.mod` | `go X.Y.Z` directive (source of truth) |
| `Dockerfile` | `FROM golang:X.Y.Z-alpine` (must match manually) |
| GitHub Actions | `go-version-file: go.mod` (automatic) |
| GoReleaser | Uses whatever Go is on the CI runner (automatic) |
| Makefile | `GOTOOLCHAIN=off` prevents silent auto-download |

### Updating the Go version

1. Edit the `go` directive in `go.mod` (e.g., `go 1.24.7`).
2. Update the `FROM golang:X.Y.Z-alpine` tag in `Dockerfile` to match.
3. Run `go mod tidy && make test`.

No workflow edits needed — CI reads `go.mod`.

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
- Keep scenario or policy adjustments near `pkg/validate`/`policy/bundles/ops-v0.1` and the `examples/` fixtures.

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
