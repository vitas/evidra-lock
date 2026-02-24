# Architecture

```mermaid
flowchart LR
  codex(AI client / CLI) --> mcp(evidra-mcp server)
  mcp --> validate(Validate service)
  validate --> runtime(OPA runtime / policy)
  runtime --> evidence(Evidence store)
  validate -->|records decision| evidence
```

## Module Responsibilities
- `cmd/evidra-mcp`: exposes the MCP `validate` tool plus `get_event` resource and wires requests to the shared evaluation core.
- `cmd/evidra`: content-free CLI surface for offline validation, policy sim, and evidence inspection that reuses `pkg/validate`.
- `pkg/validate`: shared scenario loader + runtime runner that evaluates the OPA bundle (default `policy/bundles/ops-v0.1`) and writes evidence records.
- `pkg/scenario`: scenario schema + loader (plan files, manifests, or explicit action lists) used by all entry points.(for example, Terraform plan, K8s manifest)
- `pkg/policy` + `pkg/runtime`: wrap the OPA engine and policy loader for the single decision contract.
- `pkg/evidence`: append-only store, manifest/segment helpers, and resource link generation for the MCP evidence API.
