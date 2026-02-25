# Contributing

## Prerequisites

- Go 1.24+ (exact version pinned in `go.mod`)
- OPA CLI (`opa`) for policy tests
- `git`

## Development Workflow

1. Fork and create a feature branch.
2. Make focused changes with tests.
3. Run local checks:

```bash
make fmt       # format Go code
make lint      # run linters
make test      # run all Go tests
```

4. Open a pull request.

## Adding a Policy Rule

1. Create a new `.rego` file in `policy/bundles/ops-v0.1/evidra/policy/rules/`.
   - Naming convention: `deny_<name>.rego` or `warn_<name>.rego`.
2. The rule must produce `deny["domain.rule_name"]` or `warn["domain.rule_name"]`.
   - Rule IDs use `domain.invariant_name` format: `k8s.privileged_container`, `ops.mass_delete`, `aws_iam.wildcard_policy`.
3. If the rule needs tunable parameters, add them to `evidra/data/params/data.json` with a `by_env` entry. Access values in the rule via `resolve_param` / `resolve_list_param`.
4. Add remediation hints to `evidra/data/rule_hints/data.json`, keyed by the canonical rule ID. Include 1-3 actionable strings.
5. Write OPA tests in `policy/bundles/ops-v0.1/tests/`.

Example rule structure:

```rego
package evidra.policy.rules

import data.evidra.policy.defaults.has_tag
import data.evidra.policy.defaults.action_namespace

deny["domain.rule_name"] {
    action_namespace == "tool.operation"
    # rule logic here
    not has_tag("bypass_tag")
}
```

## Running Tests

**Go tests** (all packages):

```bash
go test ./...
go test -race ./...                     # with race detector (CI requirement)
go test ./pkg/policy ./pkg/validate     # core packages only
go test -run TestFoo ./pkg/validate     # single test
```

**OPA policy tests:**

```bash
opa test policy/bundles/ops-v0.1/ -v
```

**MCP integration tests:**

```bash
go test ./cmd/evidra-mcp/test/...
```

## Code Style

- `gofmt` is the formatter. Run `make fmt` before committing.
- Keep changes minimal and deterministic.
- Preserve existing behavior unless the PR explicitly changes it.
- Avoid introducing heavy dependencies.

## Pull Requests

PRs should include:

- Problem statement
- Scope and non-goals
- Files changed
- Validation steps

Small, focused PRs are preferred over large refactors.

## Go Toolchain

`go.mod` is the single source of truth for the Go version. CI reads it automatically. When updating:

1. Edit the `go` directive in `go.mod`.
2. Update the `FROM golang:X.Y.Z-alpine` tag in `Dockerfile` to match.
3. Run `go mod tidy && make test`.
