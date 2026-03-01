---
name: evidra-infra-safety
description: Use this skill for infrastructure mutation requests such as deploy, apply, delete, patch, terraform apply or destroy, helm install or upgrade, argocd sync, evidra validate calls, and any policy check before execution. Enforce validate before mutations, skip read-only operations, fail closed on deny or validation errors, explain reasons and hints to the user, and retrieve audit evidence with get_event.
metadata:
  author: SameBits
  version: 1.0.0
  mcp-server: evidra-mcp
---

# Evidra Infra Safety

Use this skill to apply Evidra as a strict safety gate before infrastructure mutations.

## Core Rule

1. Call `validate` before every mutating infrastructure action.
2. Skip validation only for read-only operations: `get`, `describe`, `list`, `plan`, `show`, `diff`, `status`.
3. Fail closed. If validation denies, errors, or cannot be reached, do not proceed with execution.

## Validate Call Contract

The `validate` tool input is an object with required fields:

- `actor` object with required keys `type`, `id`, `origin`
- `tool` string
- `operation` string
- `params` object
- `context` object

Use these actor conventions unless your runtime needs different values:

- `actor.type`: `agent`
- `actor.id`: agent identifier such as `claude`, `codex`, or `gemini`
- `actor.origin`: `mcp`, `cli`, or `api`

### Example: Deploy to production

```json
{
  "actor": { "type": "agent", "id": "claude", "origin": "mcp" },
  "tool": "kubectl",
  "operation": "apply",
  "params": {
    "target": { "namespace": "production" },
    "payload": {
      "resource": "deployment",
      "containers": [{ "image": "nginx:1.27.0", "resources": { "limits": { "cpu": "500m", "memory": "256Mi" } } }]
    },
    "risk_tags": []
  },
  "context": { "environment": "production", "source": "agent" }
}
```

### Example: Delete in kube-system

```json
{
  "actor": { "type": "agent", "id": "claude", "origin": "mcp" },
  "tool": "kubectl",
  "operation": "delete",
  "params": {
    "target": { "namespace": "kube-system" },
    "payload": { "resource": "pod", "selector": "app=all" },
    "risk_tags": []
  },
  "context": { "environment": "production", "source": "agent" }
}
```

### Example: Terraform destroy

```json
{
  "actor": { "type": "agent", "id": "codex", "origin": "mcp" },
  "tool": "terraform",
  "operation": "destroy",
  "params": {
    "payload": { "destroy_count": 12, "resource_types": ["aws_instance", "aws_security_group"] },
    "risk_tags": []
  },
  "context": { "environment": "production", "source": "ci" }
}
```

## Response Handling

1. If `decision.allow` is `true`, proceed with execution and keep `event_id` for audit output.
2. If `decision.allow` is `false`, stop execution. Show the user `risk_level`, `reason`, `reasons`, `hints`, and `rule_ids`.
3. If validation returns an error or the tool is unreachable, stop execution and report that validation did not complete.

Never retry the same denied mutation without changed parameters or explicit user guidance.

## Evidence Retrieval

Use `get_event` when a user asks why a change was blocked or needs an audit record.

Call shape:

```json
{
  "event_id": "evt_01JEXAMPLE"
}
```

Return the fetched record and summarize key fields: decision, risk, rules, reasons, hints, actor, tool, operation, timestamp.

## Policy Reference

For the full list of policy rules, see `references/policy-rules.md`.
