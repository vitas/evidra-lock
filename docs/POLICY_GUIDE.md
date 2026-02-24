# Policy Guide

Evidra v0.1 uses the ops policy bundle:
- OPA bundle: `policy/bundles/ops-v0.1/`
- Policy rules: `policy/bundles/ops-v0.1/evidra/policy/rules/`
- Data params: `policy/bundles/ops-v0.1/evidra/data/params/data.json`
- Rule hints: `policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json`

Rego logic is stable; local customization should happen in the data files under `evidra/data/`.

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
  "context": {},
  "environment": "production | staging | ..."
}
```

The `environment` field is forwarded to OPA as `input.environment` and used by `resolve_param`/`resolve_list_param` helpers to look up environment-specific thresholds and lists.

## 2) Policy Output

Policy returns:

```json
{
  "allow": true,
  "risk_level": "low | medium | high",
  "reason": "string"
}
```

`allow=false` blocks execution in enforce mode. The policy records categorize risk as:
- `low` when no breakglass/exemption tags appeared,
- `medium` when breakglass tags are present but no denial occurred,
- `high` when a deny was emitted or the evaluation failed.

## 3) Data-Driven Configuration

Prefer data file changes over Rego edits for routine tuning.

Params are stored in `policy/bundles/ops-v0.1/evidra/data/params/data.json` using the `by_env` model:

```json
{
  "ops.mass_delete.max_deletes": {
    "by_env": { "default": 5, "staging": 10 }
  }
}
```

Common edits:
1. Adjust thresholds (e.g., `ops.mass_delete.max_deletes`) per environment.
2. Add/remove restricted or protected namespaces.
3. Update remediation hints in `evidra/data/rule_hints/data.json`.

## 4) Environment Context

Pass environment as an opaque label via `--environment` flag or `EVIDRA_ENVIRONMENT` env var. Policy rules use `resolve_param` / `resolve_list_param` helpers to look up environment-specific values from params data, falling back to `"default"` when no environment match exists.

## 5) Local Policy Testing

1. `go build -o ./bin/evidra ./cmd/evidra`
2. `./bin/evidra validate examples/terraform_plan_pass.json`
3. Look for `Decision: PASS` and `Reason` lines in the output.
4. `opa test policy/bundles/ops-v0.1/ -v` to run OPA-level tests.

## 6) Learn OPA

Useful resources to search:
- Open Policy Agent: Rego Language Reference
- OPA Playground
- `opa test` command reference
