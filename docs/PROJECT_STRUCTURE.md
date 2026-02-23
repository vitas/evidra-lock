# Project Structure & Module Map

This repository is organized around the Evidra v1-slim CLI/evaluator surface plus the supporting policy, registry, and evidence subsystems. The following sections summarize the major directories and their build/test scope.

## Repository Layout

- `cmd/` – CLI binaries (`evidra` plus any policy simulation helpers). Each command lives under `cmd/<name>` and is wired to the core packages.
- `bundles/` – packaged scenarios; the Ops bundle now only carries the scenario/schema helpers and no validator/evaluator stack.
- `docs/` – end-user and contributor guidance. Key files:
  - `docs/QUICKSTART.md`: How to run `evidra validate` on Terraform/Kubernetes fixtures.
  - `docs/policy.md`: Policy input/output contract you just added.
  - `docs/advanced.md`: MCP/advanced concepts moved out of the core narrative.
- `files/` – (if exists) additional assets.
- `pkg/` – core Go libraries. See section below.
- `policy/` – structured policy profile (`policy/profiles/ops-v0.1`):
  - `policy.rego` shim exporting `data.evidra.policy.decision`.
  - `policy/decision.rego` (aggregator) + `policy/rules/*.rego`.
  - `data.json` for thresholds/hints.
- `data/` – evidence store and staging (must be persisted by evidence tests and CI).
- `packs/`/`internal/` – helper libraries/tools referenced by other systems (CLI, registry).

## Package Modules

- `pkg/invocation`: canonical `ToolInvocation` shape plus validation.
- `pkg/policysource`: local file loader for policy modules and data; used by runtime and tests to load `policy/profiles/ops-v0.1`.
- `pkg/policy`: OPA engine wrapper that evaluates the `data.evidra.policy.decision` query and maps the result into Go `Decision` structs (allow/risk/reason/hints/hits).
- `pkg/runtime`: legacy runtime evaluator that loads policy+data; kept for backward compatibility but the v1 core now lives in `pkg/validate`.
- `pkg/validate`: single evaluation core wrapping `bundles/ops` that powers both CLI validation and MCP execution, including evidence recording.
- `pkg/config`: shared resolver for `--policy`, `--data`, and `--evidence-dir` flags plus `EVIDRA_*` env vars so both binaries use the same paths.
- `internal/advanced/registry`: legacy tool registry and validation helpers kept for advanced pack-based experimentation.
- `pkg/evidence`: append-only evidence store that records policy hits, hints, and decision metadata.
- `pkg/evidence`: append-only evidence store and helper functions for generating resource links/manifests for MCP clients.
- `pkg/mcpserver`: MCP adapter that receives `ToolInvocation`, runs the core decision/evidence flow, and exposes tools via MCP.
- `internal/advanced/engine`: legacy execution engine that routes invocations through registry, policy, and execution results (advanced flow).
- `pkg/packs`: pack loading utilities used by bundles/ops and tests.

## Build/Test Notes

- `go test ./...` covers every module; special builds (e.g., bundles/ops) include CLI integration tests.
- Policy-specific tests live under `policy/profiles/ops-v0.1/policy/tests`; run via `opa test policy/profiles/ops-v0.1`.
- QA commands (e.g., `make evidra-demo`) live in the root `Makefile`.

## Advanced / Legacy namespaces

- `internal/advanced` now hosts the legacy `engine` and `registry` modules. Treat those packages as reference material for future advanced flows; they are not used by `cmd/evidra` or `cmd/evidra-mcp` in v1.

## Single Source of Truth

Everyone edits policy under `policy/profiles/ops-v0.1`. The Go runtime, bundles, and CLI resolve policy/data from this profile by default (unless overridden via `--policy`/`--data` or `EVIDRA_POLICY_PATH`/`EVIDRA_DATA_PATH`). No other directories should be treated as authoritative.
