---

# Evidra Architecture Flow v0.1

This document defines the deterministic execution flow of Evidra v0.1.

Evidra consists of three core components:

- Tool Registry
- Policy Engine (OPA)
- Evidence Engine

Adapters (MCP, CLI, API) are transport layers and must not contain business logic.

---

## 1. Execution Flow

For every execution attempt, the following strict order must be enforced:

1. Adapter receives ToolInvocation.
2. Registry validates:
   - Tool exists.
   - Operation is supported.
   - Input shape matches tool schema.
3. If registry validation fails:
   - Create EvidenceRecord with status = denied.
   - Return error.
4. If registry validation passes:
   - Invoke Policy Engine (OPA).
5. If policy decision allow == false:
   - Create EvidenceRecord with status = denied.
   - Include policy reason.
   - Return error.
6. If policy decision allow == true:
   - Execute tool via registered executor.
   - Capture execution result.
   - Create EvidenceRecord with status = success or failed.
7. Return execution result to caller.

---

## 2. Architectural Constraints

- Adapters must not execute tools directly.
- Registry must not evaluate policy.
- Policy must not execute tools.
- Evidence must not evaluate policy.
- No component may bypass another component.

---

## 3. Determinism Rule

Given identical ToolInvocation and identical system state:

- Registry decision must be identical.
- Policy decision must be identical.
- Evidence record structure must be identical (except timestamp and hash linkage).

---

## 4. Failure Handling

- If evidence write fails, execution must be treated as failed.
- If policy evaluation fails, execution must be denied.
- If registry validation fails, execution must be denied.

---

## 5. Non-Goals (v0.1)

- No parallel execution guarantees.
- No distributed coordination.
- No multi-node consistency model.
- No remote evidence replication.

v0.1 is strictly single-instance, deterministic execution control.
