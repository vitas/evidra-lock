# Project Structure & Module Map

This repository is organized around the Evidra v1-slim CLI/evaluator surface plus the supporting policy and evidence subsystems. The following sections summarize the major directories and their build/test scope.

## Repository Layout

- `cmd/` -- CLI binaries (`evidra` plus `evidra-mcp`). Each command lives under `cmd/<name>` and is wired to the core packages.
- `docs/` -- end-user and contributor guidance. Key files:
  - `docs/QUICKSTART.md`: How to run `evidra validate` on plan/manifest fixtures.
  - `docs/policy.md`: Policy input/output contract and OPA bundle layout.
  - `docs/advanced.md`: MCP/advanced concepts moved out of the core narrative.
- `pkg/` -- core Go libraries. See section below.
- `policy/` -- OPA bundle (`policy/bundles/ops-v0.1`):
  - `.manifest` with bundle revision, roots, and profile name.
  - `evidra/policy/` -- `decision.rego` (aggregator), `defaults.rego` (helpers including `resolve_param`), `policy.rego` (shim), and `rules/*.rego`.
  - `evidra/data/params/data.json` -- data-driven thresholds and lists with `by_env` maps.
  - `evidra/data/rule_hints/data.json` -- remediation hints keyed by canonical rule IDs.
  - `tests/` -- OPA test suite per rule.
- `examples/` -- scenario fixtures for testing and demos.

## Package Modules

- `pkg/invocation`: canonical `ToolInvocation` shape plus validation; includes `Environment` field for forwarding environment labels to OPA.
- `pkg/bundlesource`: OPA bundle loader that reads `.manifest`, walks the bundle directory for `.rego` modules and `data.json` files, and reconstructs the namespaced data tree; implements `runtime.PolicySource`.
- `pkg/policysource`: local file loader for policy modules and data; used by `policy sim` subcommand for individual `.rego` + `data.json` files.
- `pkg/policy`: OPA engine wrapper that evaluates the `data.evidra.policy.decision` query and maps the result into Go `Decision` structs (allow/risk/reason/hints/hits); forwards `input.environment` when set.
- `pkg/runtime`: runtime evaluator that loads policy+data via `PolicySource` interface (with `BundleRevision()` and `ProfileName()` methods); stamps bundle metadata on every decision.
- `pkg/scenario`: scenario schema and loader shared by the CLI and MCP entrypoints.
- `pkg/validate`: single evaluation core that uses `pkg/scenario` for scenario loading and drives policy evaluation plus evidence recording for both CLI validation and MCP execution.
- `pkg/config`: shared resolver for `--bundle` and evidence store flags (`--evidence-store` alias `--evidence-dir`) plus `EVIDRA_*` env vars so both binaries use the same paths.
- `pkg/evidence`: append-only evidence store that records policy hits, hints, and decision metadata; evidence records include `BundleRevision`, `ProfileName`, `EnvironmentLabel`, and `InputHash` fields.
- `pkg/mcpserver`: MCP adapter that receives `ToolInvocation`, runs the core decision/evidence flow, and exposes tools via MCP.

## Build/Test Notes

- `go test ./...` covers every module.
- Policy-specific tests live under `policy/bundles/ops-v0.1/tests/`; run via `opa test policy/bundles/ops-v0.1/ -v`.
- QA commands (e.g., `make evidra-demo`) live in the root `Makefile`.

## Single Source of Truth

Everyone edits policy under `policy/bundles/ops-v0.1`. The Go runtime and CLI resolve policy from this bundle by default (unless overridden via `--bundle`/`EVIDRA_BUNDLE_PATH`). No other directories should be treated as authoritative.
