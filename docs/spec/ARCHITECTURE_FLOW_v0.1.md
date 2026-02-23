---

# Evidra Architecture Flow v0.1

This document defines the deterministic validation flow of Evidra v1-slim.

Evidra consists of the following layers:

- CLI/MCP adapters (`cmd/evidra`, `cmd/evidra-mcp`).
- Shared evaluation core (`pkg/validate`, `pkg/runtime`, `pkg/policy`).
- Evidence store (`pkg/evidence`).

Adapters only normalize input and present structured results; all policy
logic lives inside the core.

---

## 1. Execution Flow

1. Adapter receives a file or ToolInvocation-like payload.
2. `pkg/validate` loads/normalizes the scenario via `pkg/scenario`.
3. The runtime (`pkg/runtime` + `pkg/policy`) evaluates the policy profile.
4. The core records the decision, rule IDs, hints, and evidence via `pkg/evidence`.
5. The adapter surfaces PASS/FAIL plus rule IDs, hints, and evidence ID, ensuring `--explain`/`--json` outputs match.

---

## 2. Architectural Constraints

- The CLI and MCP adapters must not duplicate policy logic.
- Policy evaluation must remain deterministic (no side effects).
- Evidence writes occur before returning a response to the caller.
- No component may bypass the validate → policy → evidence flow.

---

## 3. Determinism Rule

With identical input and system state:

- Policy decisions must stay identical.
- Evidence records must remain identical except for timestamp/hash.
- CLI and MCP outputs must match the shared decision/hints/hits.

---

## 4. Failure Handling

- If policy evaluation denies the change, return FAIL and record denied evidence.
- If evidence writing fails, report an internal error and block the request.
- Validation failures propagate structured errors with rule IDs, hints, and reasons.

---

## 5. Non-Goals

- No execution of arbitrary commands.
- No registry/index beyond the bundled policy profile.
- No external pack runtime or plugin orchestrators.
