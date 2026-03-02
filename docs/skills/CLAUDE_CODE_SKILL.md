# Claude Code Skill: Evidra MCP Validation

Operational guidance for Claude Code clients using Evidra MCP (`validate`, `get_event`).

## 1) When to Call `validate`
- Call `validate` before destructive or privileged operations.
- Typical examples: `kubectl apply/delete`, `terraform apply/destroy`, `helm upgrade/uninstall`, `argocd sync`.

## 2) Request Shape
- `actor.type`: security classifier (`human` | `agent` | `ci`).
- `actor.origin`: transport (`mcp` | `cli` | `api`).
- `context.source`: optional metadata only (not a security classifier).
- Kubernetes payload: native manifest or flat schema is accepted; server canonicalizes internally.

Minimal request skeleton:

```json
{
  "actor": {"type": "agent", "id": "claude-code", "origin": "mcp"},
  "tool": "kubectl",
  "operation": "apply",
  "params": {
    "payload": {
      "kind": "Deployment",
      "metadata": {"namespace": "default"}
    }
  },
  "context": {"source": "interactive-session"}
}
```

## 3) Deny Handling
- If `allow=false`: STOP.
- Show reasons and hints to the user.
- Do not retry unchanged input.

## 4) Insufficient Context Handling
- If deny/hints indicate missing context, ask user for required fields.
- Re-run `validate` only after input changes.

One-loop pattern:

```json
{
  "step": "validate",
  "if_deny_with_missing_context": "ask_for_required_fields_then_retry_once_with_updated_payload"
}
```

## 5) `get_event` Usage
- Use `get_event` with `event_id` returned by `validate` to retrieve immutable evidence/audit details.

