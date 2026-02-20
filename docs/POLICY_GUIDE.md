# Policy Guide

## 1) Policy Input

Evidra policy evaluates canonical `ToolInvocation` input:

```json
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
```

## 2) Policy Output Contract

Policy must return:

```json
{
  "allow": true,
  "risk_level": "low | medium | high | critical",
  "reason": "string"
}
```

Notes:
- `allow=false` blocks execution in enforce mode.
- `reason` should come from controlled reason codes.

## 3) Environment Control

Set execution context from caller input:
- `context.environment`: `dev` or `prod`
- `context.cluster`: `local` or `remote` (if used by policy)

Policies should use these fields for risk and allow decisions, especially for write operations.

## 4) Typical Rules

Common operations policy shape:
- Read operations: allow with `low` risk.
- Write operations in `dev`: allow with `high` risk.
- Write operations in `prod`: `critical` risk or explicit deny.

## 5) Local Policy Testing

Use policy simulator:

```bash
go run ./cmd/evidra-policy-sim --policy ./policy/policy.rego --data ./policy/data.json --input ./examples/invocations/allowed_kubectl_get_dev.json
```

If you maintain Rego tests in policy folders, run `opa test` over those directories.

## 6) Learn OPA

Useful resources to search:
- Open Policy Agent: Rego Language Reference
- OPA Playground
- `opa test` command reference
