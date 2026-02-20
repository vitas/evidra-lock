# argocd-basic Pack

Minimal Argo CD operations pack for controlled runtime use.

Included operations:
- `argocd/app-list`
- `argocd/app-get`
- `argocd/app-sync`
- `argocd/app-rollback`

Load with ops profile defaults or explicitly:

```bash
EVIDRA_PACKS_DIR=./packs/_core/ops
```

Example ToolInvocation (`app-sync`):

```json
{
  "actor": {"type": "human", "id": "ops-user", "origin": "mcp"},
  "tool": "argocd",
  "operation": "app-sync",
  "params": {"app": "payments-api"},
  "context": {"environment": "dev"}
}
```

Policy guidance under `policy/` uses `input.context.environment` to classify read/write risk.
