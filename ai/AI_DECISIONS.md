# AI Decisions Log

## Purpose
Record all architectural and refactoring decisions influenced by AI, including rationale and impact.

## 2026-02-19 - Samebits Core v0.1 Evidence Ledger
- Decision: Use a generic `EvidenceRecord` schema with neutral fields (`id`, `timestamp`, `actor`, `action`, `subject`, `details`, `prev_hash`, `hash`) because the provided schema placeholder was not populated.
- Rationale: Preserve domain neutrality while providing enough structure for immutable chain linking and audit replay.
- Decision: Compute record hash from canonical JSON that excludes `hash` by marshaling a dedicated canonical struct.
- Rationale: Enforces stable hashing semantics and prevents self-referential hash inclusion.
- Decision: Persist records as append-only JSONL at `./data/evidence.log`; set `prev_hash` to prior record hash during append.
- Rationale: Aligns with immutable ledger requirements and enables sequential chain validation.
- Decision: Validate chain by recomputing each hash and verifying `prev_hash` linkage from head to tail.
- Rationale: Detects both content tampering and link tampering with minimal complexity.

## 2026-02-20 - Evidra MCP v0.1 Layered Execution Flow
- Decision: Use official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) with `mcp.NewServer`, `mcp.AddTool`, and stdio transport (`mcp.StdioTransport`).
- Rationale: Satisfies official SDK requirement while keeping transport minimal.
- Decision: Introduce `pkg/mcpserver` as a transport adapter with thin handler delegating to `ExecuteService`.
- Rationale: Enforces separation of concerns and keeps business decisions out of MCP handler functions.
- Decision: Introduce `pkg/policy` with default-deny exact command matching loaded from `config/policy.yaml`.
- Rationale: Meets deterministic allow-list requirement with minimal policy surface.
- Decision: Introduce `pkg/executor` wrapper around `exec.CommandContext` with 10-second timeout, no shell invocation, and captured stdout/stderr/exit code.
- Rationale: Ensures safe command execution and deterministic output shape.
- Decision: Add `pkg/evidence.Store` thin abstraction for initialization and append/validate calls.
- Rationale: Allows dependency injection into server layer without embedding evidence logic into handlers.
- Decision: Add dependency `gopkg.in/yaml.v3` to parse policy YAML.
- Rationale: Required to support `config/policy.yaml` loading while maintaining simple, explicit parsing behavior.

## 2026-02-20 - Registry-First + OPA + Evidence Contract Refactor
- Decision: Replace generic `command/args` execution path with explicit static tool registry (`echo/run`, `git/status`) in `pkg/registry`.
- Rationale: Enforces controlled tool surface and removes generic shell-style invocation path.
- Decision: Enforce runtime flow in MCP adapter as structure validation -> registry validation (tool, operation, params) -> policy evaluation -> registered executor -> evidence write.
- Rationale: Aligns behavior with architecture flow contract and prevents component bypass.
- Decision: Replace YAML allowlist policy with embedded OPA/Rego policy engine loading `policy/policy.rego`.
- Rationale: Meets v0.1 policy contract requirements (`allow`, `reason`, default deny, deterministic evaluation).
- Decision: Align evidence schema with contract fields (`event_id`, structured actor, tool/operation/params, policy_decision, execution_result, previous_hash/hash).
- Rationale: Ensures evidence records are contract-compatible and auditable for all attempts.
- Decision: Remove old YAML policy file and generic executor package.
- Rationale: Prevent parallel policy/execution paths and avoid accidental fallback to non-contract behavior.

## 2026-02-20 - Policy Product Layer Hardening
- Decision: Introduce standardized policy reason code vocabulary in `policy/reason_codes.rego`.
- Rationale: Keeps policy decisions machine-checkable and auditable with deterministic semantics.
- Decision: Add policy templates (`dev_safe`, `regulated_dev`, `ci_agent`) under `policy/templates/`.
- Rationale: Provides minimal, testable policy variants without changing runtime execution flow.
- Decision: Add OPA policy data baseline in `policy/data.json` for high-risk operation classification.
- Rationale: Centralizes deterministic risk signals for template rules.
- Decision: Add OPA-style template test suites under `policy/tests/` and policy template usage doc `spec/POLICY_TEMPLATES.md`.
- Rationale: Strengthens policy quality and onboarding without adding runtime complexity.

## 2026-02-20 - Policy Risk Metadata Extension
- Decision: Extend policy decision output contract to include deterministic `risk_level` with controlled values (`low`, `medium`, `high`, `critical`).
- Rationale: Adds governance metadata without changing allow/deny semantics or execution flow.
- Decision: Extend evidence `policy_decision` payload to store `risk_level` pass-through from policy.
- Rationale: Ensures risk classification is auditable in immutable evidence records.
- Decision: Keep Go runtime behavior unchanged except policy output validation/pass-through for `risk_level`.
- Rationale: Meets scope constraint of policy-driven metadata only, without introducing new workflow behavior.

## 2026-02-20 - Offline Policy Simulation CLI
- Decision: Add standalone local simulator command `cmd/evidra-policy-sim` using only `flag` and existing `pkg/policy` + `pkg/invocation`.
- Rationale: Enables deterministic regulated-developer policy checks without MCP, execution, or evidence side effects.
- Decision: Extend policy loader with `LoadFromFiles(policyPath, dataPaths)` and keep `LoadFromFile` as compatibility wrapper.
- Rationale: Reuses existing policy engine while supporting optional external data file input for simulations.

## 2026-02-20 - Debt Cleanup: Remove Generic Command Path
- Decision: Keep only registry-based execution with explicit tool operations and remove reliance on generic command patterns.
- Rationale: Aligns runtime strictly with v0.1 spec contracts and controlled tool surface.
- Decision: Update `echo/run` executor to call system `echo` and `git/status` executor to use `git -C <path> status --porcelain`.
- Rationale: Matches required tool executor behavior while remaining deterministic and scoped.

## 2026-02-20 - Enforce/Observe Execution Modes
- Decision: Add runtime mode switch (`enforce` default, `observe`) via `EVIDRA_MODE` env and keep flow ordering unchanged.
- Rationale: Supports advisory policy evaluation for regulated workflows without bypassing registry validation.
- Decision: In observe mode, policy is always evaluated but does not block execution; policy decision is recorded with `advisory=true`.
- Rationale: Preserves deterministic governance metadata while maintaining strict tool-surface control.
- Decision: Add env-based runtime path config (`EVIDRA_POLICY_PATH`, optional `EVIDRA_POLICY_DATA_PATH`, `EVIDRA_EVIDENCE_PATH`).
- Rationale: Enables controlled local/runtime configuration without adding config-file complexity.

## 2026-02-20 - Core Interface Boundaries for Policy/Evidence
- Decision: Add transport-agnostic core interfaces (`core.PolicySource`, `core.PolicyEngine`, `core.EvidenceStore`) and wire runtime through these boundaries.
- Rationale: Preserves existing behavior while enabling future server-driven policy/evidence implementations without rewriting execution flow.
- Decision: Add local file-backed policy source with deterministic `PolicyRef()` hash and include `policy_ref` in every evidence record.
- Rationale: Improves forensic traceability and keeps local deployment deterministic.
