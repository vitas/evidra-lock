---

# Tool Registry Contract v0.1

This document defines the strict interface between Evidra Core and the Tool Registry.

The Tool Registry is responsible for defining which tools exist, how they are executed, and ensuring that only registered tools can be invoked.

---

## 1. Registry Model

- Only registered tools may be executed.
- Generic shell execution is prohibited.
- Tool registration is explicit and static in v0.1.
- If a tool is not registered, execution must be denied before policy evaluation.

---

## 2. Tool Definition Requirements

Each registered tool must define:

- name: Unique tool identifier.
- supported_operations: Explicit list of allowed operations.
- executor: Deterministic execution handler.
- input_schema: Structured parameter definition (no raw shell strings).

Tools must not accept arbitrary command strings.

---

## 3. Invocation Contract

A ToolInvocation must include:

{
  "actor": {...},
  "tool": "string",
  "operation": "string",
  "params": {},
  "context": {}
}

Validation order:

1. Registry verifies tool exists.
2. Registry verifies operation is supported.
3. Policy evaluation is executed.
4. Execution proceeds only if policy allow == true.

---

## 4. Determinism Requirements

- Tool execution must be deterministic given identical inputs.
- Executors must not mutate global state outside defined tool behavior.
- Side effects must be limited to the tool’s explicit responsibility.

---

## 5. Enforcement Rule

If a tool or operation is not registered:

- Execution MUST NOT proceed.
- Evidence must record the rejection reason as:
  "unregistered_tool" or "unsupported_operation".
