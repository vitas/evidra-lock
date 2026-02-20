# Evidra MCP

## Policy Simulation (offline)

Evaluate policy for a local `ToolInvocation` JSON without MCP, execution, or evidence writes.

```bash
go run ./cmd/evidra-policy-sim --policy ./policy/policy.rego --input ./examples/invocations/allowed_echo.json --data ./policy/data.json
```

Expected output:

```json
{
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_by_rule"
}
```
