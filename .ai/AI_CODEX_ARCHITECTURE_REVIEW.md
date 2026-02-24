# AI Architecture CodeX Review

Date: 2026-02-23  
Reviewer: Codex (system/software architecture review)

## 1) Executive Summary

The project has converged on a much cleaner v0.1 shape:

- `pkg/validate` is the practical decision core used by both `cmd/evidra` and `cmd/evidra-mcp`.
- Policy is structured and data-driven under `policy/profiles/ops-v0.1`.
- MCP server is validate-first (`validate` tool + `get_event`).
- Evidence is hash-chained and operationally useful.

Main risks are now complexity concentration and documentation drift, not fundamental architecture failure.

Top priorities:

1. Split `pkg/evidence/evidence.go` (1207 LOC) by responsibility.
2. Split `cmd/evidra/evidence_cmd.go` (809 LOC) by subcommand.
3. Fix policy loader path-key quirk in `pkg/policysource/local_file.go`.
4. Reconcile docs with real defaults/commands.

## 2) Scope and Method

Reviewed:

- Binaries: `cmd/evidra`, `cmd/evidra-mcp`
- Core packages: `pkg/validate`, `pkg/scenario`, `pkg/runtime`, `pkg/policy`, `pkg/evidence`, `pkg/mcpserver`, `pkg/config`, `pkg/policysource`, `pkg/invocation`
- Policy profile: `policy/profiles/ops-v0.1/**`
- Docs: `README.md`, `docs/*.md`, `AGENTS.md`

Validation snapshot:

- `go test ./...` passed.
- `go test ./... -cover` passed.
- `opa test policy/profiles/ops-v0.1` passed.

## 3) Current Architecture (Observed)

Canonical v0.1 flow is coherent:

1. Input arrives via CLI (`evidra validate`) or MCP (`validate` tool).
2. `pkg/validate` normalizes input into scenario/action form.
3. `pkg/runtime` + `pkg/policy` evaluate `data.evidra.policy.decision`.
4. Decision + metadata are persisted via `pkg/evidence`.
5. Caller receives PASS/FAIL, risk, reasons/hits/hints, evidence id.

This is the right architecture for v0.1: deterministic, local-first, explainable.

## 4) Findings by Severity

### Critical

#### C1. Evidence subsystem is a monolith with mixed concerns
- File: `pkg/evidence/evidence.go` (~1207 LOC)
- Symptoms:
  - Store mode detection (legacy + segmented), append/read/validate, manifest, forwarder cursor, lock handling, hash logic are all in one file.
  - High coupling increases regression risk and makes selective change hard.
- Risk:
  - Any modification to one concern can break others (especially chain validation and cursor flows).
- Recommendation:
  - Split into focused files without API change:
    - `store.go` (public API)
    - `append.go`
    - `read.go`
    - `chain.go`
    - `manifest.go`
    - `cursor.go`
    - `locking.go`

#### C2. Policy module path key generation is brittle
- File: `pkg/policysource/local_file.go`
- Issue:
  - `loadPolicyDir(root)` computes relative paths from `s.PolicyPath` instead of `root`.
  - When loading shim + sibling policy dir, keys may become `../...`.
- Risk:
  - Fragile module keys, harder debugging, unstable policy ref hashing semantics.
- Recommendation:
  - Use `filepath.Rel(root, path)` for directory walks.
  - Keep deterministic sorted keys for ref hashing.

### High

#### H1. Offline evidence CLI is oversized
- File: `cmd/evidra/evidence_cmd.go` (~809 LOC)
- Risk:
  - High cognitive load, poor testability of subcommands, merge conflict hotspot.
- Recommendation:
  - Split by subcommand (`verify`, `export`, `violations`, `cursor`) and shared helpers.

#### H2. Documentation drift against code reality
- Key examples:
  - `docs/evidence.md` and `docs/EVIDENCE_GUIDE.md` still describe `./data/evidence` default; code uses `~/.evidra/evidence` via `pkg/config`.
  - `docs/INDEX.md` references `docs/TOOL_PACKS.md` which is not present.
  - `policy/profiles/ops-v0.1/README.md` describes old data knobs (`operation_classes`, `overrides`) not in current `data.json`.
  - `AGENTS.md` suggests `opa test policy/profiles/ops-v0.1/policy/...`; practical command is `opa test policy/profiles/ops-v0.1`.
- Risk:
  - Operator confusion, onboarding friction, wrong runbooks.
- Recommendation:
  - Create one docs baseline pass and align defaults/commands with current code.

### Medium

#### M1. Test coverage is uneven on core contract packages
- From `go test ./... -cover`:
  - `pkg/policy`: 14.3%
  - `pkg/config`: 0.0%
  - `pkg/mcpserver`: 46.3%
- Risk:
  - Contract regressions can slip through despite broad test count.
- Recommendation:
  - Add targeted contract tests:
    - `pkg/config`: flag/env precedence matrix
    - `pkg/policy`: invalid decision shape/risk-level enforcement
    - `pkg/mcpserver`: validate output schema stability

#### M2. Legacy store mode still adds complexity to v0.1
- Files: `pkg/evidence/evidence.go`, `cmd/evidra/evidence_cmd.go`
- Observation:
  - Legacy vs segmented branches remain throughout code paths.
- Recommendation:
  - Keep read compatibility, but consider writing only segmented mode and isolating legacy logic into a separate adapter file.

#### M3. Minor contract duplication across CLI/MCP presentation
- Files: `cmd/evidra/main.go`, `pkg/mcpserver/server.go`
- Observation:
  - Similar output concerns are handled in two places.
- Recommendation:
  - Keep behavior, but centralize shared response-shaping utilities if divergence starts.

### Low

#### L1. Generated-by-AI inline comments in core types add noise
- File: `pkg/invocation/invocation.go`
- Recommendation:
  - Replace provenance comments with concise engineering comments focused on behavior.

## 5) Module-by-Module Assessment

| Module | Role | Architecture Quality | Complexity | Reuse |
|---|---|---|---|---|
| `pkg/validate` | Single decision core | Strong | Medium | High |
| `pkg/scenario` | Input detection/normalization | Good | Medium | High |
| `pkg/policy` + `pkg/runtime` | OPA evaluation and wiring | Good | Medium | High |
| `pkg/mcpserver` | MCP transport adapter | Good | Medium | Medium-High |
| `pkg/evidence` | Evidence store + chain | Functionally strong | High | Medium |
| `cmd/evidra` | Offline UX + advanced evidence tools | Mixed | High | Medium |
| `cmd/evidra-mcp` | Server bootstrap/config | Good | Low | Medium |
| `pkg/config` | Path/env resolution | Good | Low | High |
| `pkg/policysource` | Policy/data loading | Good but brittle path key logic | Low-Medium | High |

## 6) Reuse and Simplification Opportunities

1. Keep `ToolInvocation` (yes, still needed)
- It is actively used in:
  - `pkg/mcpserver` tool input
  - `pkg/validate.EvaluateInvocation`
  - `cmd/evidra policy sim`
  - runtime/policy contract mapping
- It is a useful canonical boundary type and should remain.

2. Preserve one policy/evidence contract end-to-end
- Current decision fields (`allow`, `risk_level`, `reason`, `reasons`, `hits`, `hints`) are consistent across policy, Go runtime, MCP, and CLI.
- Keep this as a hard contract and resist ad-hoc response variants.

3. Narrow docs to one source per topic
- Current overlap (`docs/policy.md` vs `docs/POLICY_GUIDE.md`, `docs/evidence.md` vs `docs/EVIDENCE_GUIDE.md`) risks drift.
- Keep one canonical contract doc per domain, and make others brief “how-to” pages.

## 7) Suggested Execution Plan (No Feature Additions)

### Phase 1 (low risk, high ROI)
1. Fix `pkg/policysource/local_file.go` relative path base.
2. Align docs defaults/commands with code (`~/.evidra/evidence`, correct `opa test` path).
3. Remove stale policy profile README content.

### Phase 2 (structural cleanup)
1. Split `pkg/evidence/evidence.go` by concerns (no behavior change).
2. Split `cmd/evidra/evidence_cmd.go` by subcommand files.

### Phase 3 (contract hardening)
1. Add focused tests for `pkg/config`, `pkg/policy`, and MCP output schema.
2. Decide legacy evidence mode lifecycle (retain read-compat, isolate or retire write path).

## 8) Final Architectural Verdict

The codebase is now viable as a v0.1 validate-first product. The major technical risk is concentrated complexity in evidence and command modules, plus docs drift that can mislead operators. With targeted decomposition and contract-focused tests, the system can stay simple while remaining extensible.
