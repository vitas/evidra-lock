# helm-basic Pack

This pack registers a minimal controlled Helm tool surface:
- `helm/version`
- `helm/list`
- `helm/status`
- `helm/upgrade`

Load locally:

```bash
EVIDRA_PACKS_DIR=./packs/_core
```

Example ToolInvocation (`helm/status`):

```json
{
  "actor": {"type": "human", "id": "dev-user", "origin": "mcp"},
  "tool": "helm",
  "operation": "status",
  "params": {"release": "payments-api", "namespace": "payments"},
  "context": {"environment": "dev"}
}
```

Example ToolInvocation (`helm/upgrade`):

```json
{
  "actor": {"type": "human", "id": "dev-user", "origin": "mcp"},
  "tool": "helm",
  "operation": "upgrade",
  "params": {
    "release": "payments-api",
    "chart": "./charts/payments-api",
    "namespace": "payments"
  },
  "context": {"environment": "prod"}
}
```

Policy guidance is provided under `policy/` and uses `input.context.environment`.
