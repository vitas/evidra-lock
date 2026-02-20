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

Response includes `event_id`.

## get_event Payload

```json
{
  "event_id": "evt-..."
}
```

Use this to retrieve immutable evidence records after execution.

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
