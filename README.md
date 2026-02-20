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

## ToolInvocation Examples

`echo/run`:

```json
{
  "actor": {"type": "human", "id": "dev-user", "origin": "cli"},
  "tool": "echo",
  "operation": "run",
  "params": {"text": "hello"},
  "context": {}
}
```

## Execution Modes

- `enforce` (default): policy deny blocks execution.
- `observe`: policy is evaluated but does not block execution; decisions are advisory.

Example:

```bash
EVIDRA_MODE=observe ./evidra-mcp
```

Observe mode does **not** bypass registry validation. Unknown tools and unsupported operations are still denied.

`git/status`:

```json
{
  "actor": {"type": "human", "id": "dev-user", "origin": "cli"},
  "tool": "git",
  "operation": "status",
  "params": {"path": "."},
  "context": {}
}
```
