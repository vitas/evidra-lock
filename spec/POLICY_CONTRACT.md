---

# Policy Contract v0.1

This document defines the strict interface between Evidra Core and the Policy Engine (OPA/Rego).

The policy engine is responsible for evaluating whether a tool invocation is allowed.
It must be deterministic and side-effect free.

---

## 1. Evaluation Model

- Default decision: deny
- Policy evaluation must be pure (no external side effects).
- Policy must not perform execution.
- Policy must not mutate input.

---

## 2. Input Schema

The following JSON object is provided to OPA as `input`:

{
  "actor": {
    "type": "human | ai | system",
    "id": "string",
    "origin": "mcp | cli | api | unknown"
  },
  "tool": "string",
  "operation": "string",
  "params": {},
  "context": {}
}

### Field Definitions

- actor: Identity of the execution initiator.
- tool: Registered tool name.
- operation: Subcommand or operation name.
- params: Structured parameters (must not contain raw shell strings).
- context: Optional structured execution context (environment, metadata).

---

## 3. Required Policy Output

Policy evaluation must return:

{
  "allow": true | false,
  "reason": "string"
}

### Requirements

- allow must always be defined.
- reason must always be defined.
- reason must be human-readable and deterministic.
- No other fields are required in v0.1.

---

## 4. Determinism Requirements

For identical input, policy evaluation must always produce identical output.

Time-based, random, or external state-based decisions are out of scope for v0.1.

---

## 5. Enforcement Rule

Execution MUST NOT proceed unless:

- allow == true

If allow == false:
- Execution is denied.
- Evidence must record the policy decision and reason.
