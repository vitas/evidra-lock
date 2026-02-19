# AI Policy Guide

## Purpose
This guide defines how Evidra policy is authored and evaluated for v0.1.

## Policy Language
- Policies are written in Rego and evaluated by embedded OPA.
- Policy behavior is default deny unless an explicit allow rule matches.

## Required Input Shape
Policy input must include:
- `actor`: actor metadata (who requested execution)
- `tool`: tool metadata (which registered tool is targeted)
- `operation`: requested operation (for v0.1 typically `execute`)
- `params`: operation parameters
- `context`: runtime context (environmental and request metadata)

Example shape:
```json
{
  "actor": {"id": "user-1"},
  "tool": {"name": "git"},
  "operation": "execute",
  "params": {"args": ["status"]},
  "context": {}
}
```

## Required Outputs
Policy decision output must provide:
- `allow` (bool): final permit/deny decision
- `reason` (string): deterministic explanation for the decision

## Authoring Rules
- Keep policies small and composable.
- Prefer explicit allow conditions over broad pattern matching.
- Keep reasons stable and human-readable for evidence/audit use.
