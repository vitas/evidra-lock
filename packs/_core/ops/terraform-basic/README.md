# terraform-basic Pack

Minimal Terraform operations pack for controlled runtime use.

Supported operations:
- `terraform/init`
- `terraform/plan`
- `terraform/apply`

This surface is intentionally limited in v0.1:
- no `-target`
- no `-var` / `-var-file`
- no free-form arguments

Load with ops profile defaults or explicitly:

```bash
EVIDRA_PACKS_DIR=./packs/_core/ops
```

Example ToolInvocation (`plan`):

```json
{
  "actor": {"type": "human", "id": "ops-user", "origin": "mcp"},
  "tool": "terraform",
  "operation": "plan",
  "params": {"dir": "./infra"},
  "context": {"environment": "dev"}
}
```

Example ToolInvocation (`apply`):

```json
{
  "actor": {"type": "human", "id": "ops-user", "origin": "mcp"},
  "tool": "terraform",
  "operation": "apply",
  "params": {"dir": "./infra"},
  "context": {"environment": "prod"}
}
```

Policy guidance under `policy/` uses `input.context.environment` for risk classification.
