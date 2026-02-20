# MCP Guide

MCP is the primary integration surface for agents.

Evidra MCP server intercepts invocations and enforces:
- Registry (known tools only)
- Policy (OPA decision)
- Evidence write

## Enforcement Model

- Evidra enforces guardrails only for tool invocations that pass through its MCP server.
- If an agent can run arbitrary shell commands directly, it can bypass Evidra.
- Evidra is not:
  - an OS-level hook
  - a shell wrapper
  - a Kubernetes admission controller
  - a syscall interceptor

```text
Agent
   ↓ (MCP)
Evidra MCP
   ↓
Registry → Policy (OPA) → Execution → Evidence
```

## Guarded Mode (Recommended)

Guarded Mode means:
- Agent has no direct shell/terminal tool.
- Agent only has access to MCP tools exposed by Evidra.
- `kubectl` / `helm` / `terraform` / `argocd` are reachable only through Evidra.

Guarded Mode Checklist:
- Disable generic shell tool in your agent/IDE.
- Remove direct access to `kubectl`/`terraform` from the agent sandbox (if applicable).
- Connect the agent to Evidra MCP server.
- Confirm tool calls appear in the evidence log.
- Run `evidra-evidence verify --evidence ./data/evidence`.

In dev-oriented setups, shell may remain enabled, but enforcement becomes advisory because bypass paths exist.

## MCP Tools

- `execute`
- `get_event`

## Tool Metadata and Safety Hints

Each MCP tool exposes descriptions and input schema details. Tool definitions also include safety annotations intended for UI hinting:
- read-only operations: `side_effects=none`, `risk_hint=low`
- write operations: `side_effects=writes`, `risk_hint=high`
- destructive operations: `side_effects=destructive`, `risk_hint=critical`

These are hints for UX only. Policy evaluation is the enforcement source of truth.

## execute Payload

```json
{
  "actor": {"type": "ai", "id": "agent-1", "origin": "mcp"},
  "tool": "argocd",
  "operation": "app-get",
  "params": {"app": "payments-api"},
  "context": {"environment": "prod"}
}
```

## execute Response (allowed)

```json
{
  "ok": true,
  "event_id": "evt-123",
  "policy": {
    "allow": true,
    "risk_level": "low",
    "reason": "allowed_by_rule",
    "policy_ref": "b4b6..."
  },
  "execution": {
    "status": "success",
    "exit_code": 0,
    "stdout": "...",
    "stderr": ""
  },
  "hints": [
    "Execution allowed by policy."
  ]
}
```

## execute Response (denied)

```json
{
  "ok": false,
  "event_id": "evt-124",
  "policy": {
    "allow": false,
    "risk_level": "critical",
    "reason": "policy_denied_default",
    "policy_ref": "b4b6..."
  },
  "execution": {
    "status": "denied",
    "exit_code": null,
    "stdout": "",
    "stderr": ""
  },
  "error": {
    "code": "policy_denied_default",
    "message": "execution denied by policy",
    "risk_level": "critical",
    "reason": "policy_denied_default",
    "hint": "Adjust policy rules or invocation context (for example context.environment)."
  },
  "hints": [
    "Run evidra-policy-sim to evaluate policy decisions offline."
  ]
}
```

UIs should surface `policy.risk_level`, `policy.reason`, and `event_id` for traceability and incident review.

## get_event Payload

```json
{
  "event_id": "evt-..."
}
```

## get_event Responses

On success:

```json
{
  "ok": true,
  "record": {
    "event_id": "evt-123",
    "timestamp": "2026-02-20T12:00:00Z",
    "tool": "argocd",
    "operation": "app-get",
    "hash": "..."
  }
}
```

On not found:

```json
{
  "ok": false,
  "error": {
    "code": "not_found",
    "message": "event_id not found"
  }
}
```

On chain validation failure:

```json
{
  "ok": false,
  "error": {
    "code": "evidence_chain_invalid",
    "message": "evidence chain validation failed"
  }
}
```

Use `get_event` to retrieve immutable evidence records after execution.

Offline CLI tools (`policy-sim`, `evidra-evidence`) remain available for local iteration and forensics.

## MCP Proxy / Gateway Pattern (Advanced)

An MCP proxy can sit between an agent and multiple MCP servers.

The proxy can:
- Expose only approved tools.
- Hide dangerous tools (for example, shell).
- Route tools to specific backends.
- Centralize authentication and policy.

```text
Agent
   ↓
MCP Proxy
   ↓
Evidra MCP
   ↓
Execution
```

This pattern is useful in enterprise environments with many MCP tools.
Evidra can run behind such a proxy for stronger control boundaries.

Evidra does not currently implement a full MCP proxy. It can run standalone or behind an external proxy.
