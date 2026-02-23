# Policy Guide

Evidra v0.1 uses the ops policy profile:
- `policy/profiles/ops-v0.1/policy.rego`
- `policy/profiles/ops-v0.1/data.json`

Rego logic is stable; local customization should happen in `data.json`.

## 1) Policy Input

Policy evaluates canonical `ToolInvocation`:

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

## 2) Policy Output

Policy returns:

```json
{
  "allow": true,
  "risk_level": "low | medium | high | critical",
  "reason": "string"
}
```

`allow=false` blocks execution in enforce mode.

## 3) Data-Driven Configuration

Prefer `data.json` changes over Rego edits for routine tuning.

Common edits:
1. Add/remove allowed container registries.
2. Add/remove S3 delete allowlist prefixes.
3. Adjust operation-specific allowlists defined in `data.json`.

## 4) Environment Context

Policies use request context for risk and permission decisions:
- `context.environment` (`dev` or `prod`)
- `context.cluster` (`local` or `remote`, if referenced by a rule)

## 5) Local Policy Testing

1. `go build -o ./bin/evidra ./cmd/evidra`
2. `./bin/evidra validate examples/terraform_plan_pass.json`
3. Look for `Decision: PASS` and `Reason` lines in the output.

## 6) Learn OPA

Useful resources to search:
- Open Policy Agent: Rego Language Reference
- OPA Playground
- `opa test` command reference
