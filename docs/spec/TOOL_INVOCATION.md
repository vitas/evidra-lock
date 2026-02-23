---

# Tool Invocation Contract v0.1

This document defines the canonical ToolInvocation structure used across Evidra.

ToolInvocation is the single normalized execution request format used internally,
regardless of transport adapter (MCP, CLI, API).

All adapters must convert incoming requests into this structure before
interacting with Registry, Policy, or Evidence components.

---

## 1. Canonical Structure

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

---

## 2. Field Requirements

actor:
- type is mandatory.
- id is mandatory.
- origin is mandatory.

tool:
- Must match a registered tool name exactly.
- Case-sensitive.

operation:
- Must match one of the tool’s supported_operations.
- Case-sensitive.

params:
- Must be structured JSON.
- Must not contain raw shell command strings.
- Must comply with the tool’s input schema.

context:
- Optional structured metadata.
- Must not affect execution unless explicitly defined by tool behavior.

---

## 3. Validation Order

Before policy evaluation:

1. Validate structure integrity.
2. Validate tool is registered.
3. Validate operation is supported.
4. Validate params shape.

Only after these steps may policy evaluation occur.

---

## 4. Determinism Rule

For identical ToolInvocation inputs:

- Registry validation result must be identical.
- Policy decision must be identical.
- Executor behavior must be identical (given identical system state).

---

## 5. Prohibited Patterns

The following are explicitly prohibited:

- Raw shell execution strings.
- Dynamic tool resolution.
- Implicit operation inference.
- Unstructured parameters.
- Hidden execution paths outside Registry.

All execution must pass through explicit ToolInvocation processing.
