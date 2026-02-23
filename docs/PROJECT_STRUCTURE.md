# Project Structure & Module Map

This repository is organized around the Evidra v1-slim CLI/evaluator surface plus the supporting policy and evidence subsystems. The following sections summarize the major directories and their build/test scope.

## Repository Layout

- `cmd/` – CLI binaries (`evidra` plus any policy simulation helpers). Each command lives under `cmd/<name>` and is wired to the core packages.
- Legacy packaged scenarios have been retired; scenario/schema helpers and fixtures now live under `pkg/scenario` and `examples/`.
- `docs/` – end-user and contributor guidance. Key files:
  - `docs/QUICKSTART.md`: How to run `evidra validate` on plan/manifest fixtures.
  - `docs/policy.md`: Policy input/output contract you just added.
  - `docs/advanced.md`: MCP/advanced concepts moved out of the core narrative.
- `files/` – (if exists) additional assets.
- `pkg/` – core Go libraries. See section below.
- `policy/` – structured policy profile (`policy/profiles/ops-v0.1`):
  - `policy.rego` shim exporting `data.evidra.policy.decision`.
  - `policy/decision.rego` (aggregator) + `policy/rules/*.rego`.
  - `data.json` for thresholds/hints.
- `data/` – evidence store and staging (must be persisted by evidence tests and CI).
- `packs/` and `internal/advanced/` were removed; the remaining directories focus on policy, CLI, and evidence.

## Package Modules

- `pkg/invocation`: canonical `ToolInvocation` shape plus validation.
- `pkg/policysource`: local file loader for policy modules and data; used by runtime and tests to load `policy/profiles/ops-v0.1`.
- `pkg/policy`: OPA engine wrapper that evaluates the `data.evidra.policy.decision` query and maps the result into Go `Decision` structs (allow/risk/reason/hints/hits).
- `pkg/runtime`: legacy runtime evaluator that loads policy+data; kept for backward compatibility but the v1 core now lives in `pkg/validate`.
- `pkg/scenario`: scenario schema and loader shared by the CLI and MCP entrypoints.
- `pkg/validate`: single evaluation core that uses `pkg/scenario` for scenario loading and drives policy evaluation plus evidence recording for both CLI validation and MCP execution.
- `pkg/config`: shared resolver for `--policy`, `--data`, and `--evidence-dir` flags plus `EVIDRA_*` env vars so both binaries use the same paths.
- `pkg/evidence`: append-only evidence store that records policy hits, hints, and decision metadata.
- `pkg/evidence`: append-only evidence store and helper functions for generating resource links/manifests for MCP clients.
- `pkg/mcpserver`: MCP adapter that receives `ToolInvocation`, runs the core decision/evidence flow, and exposes tools via MCP.
**Legacy:** The previous `internal/advanced` and `pkg/packs` layers were removed; no advanced helper packages remain in the v1 tree.

## Build/Test Notes

- `go test ./...` covers every module; special builds (e.g., advanced packages) include CLI integration tests.
- Policy-specific tests live under `policy/profiles/ops-v0.1/policy/tests`; run via `opa test policy/profiles/ops-v0.1`.
- QA commands (e.g., `make evidra-demo`) live in the root `Makefile`.

## Advanced / Legacy namespaces

Legacy advanced helpers were removed; all remaining packages belong to the v1 core evaluation path.

## Single Source of Truth

Everyone edits policy under `policy/profiles/ops-v0.1`. The Go runtime and CLI resolve policy/data from this profile by default (unless overridden via `--policy`/`--data` or `EVIDRA_POLICY_PATH`/`EVIDRA_DATA_PATH`). No other directories should be treated as authoritative.
