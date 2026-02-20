# Evidra - Guardrails for Operations in the AI Agent Era

Evidra intercepts agent tool invocations through MCP, evaluates each request with OPA policy, and records tamper-evident evidence for audit and incident response.

## What It Is

- An operations guardrail layer for AI-driven tool execution.
- Policy-driven control using OPA/Rego with default deny.
- Immutable forensic evidence with segmented, hash-chained logs.

## What It Is Not

- A CI/CD system.
- A Kubernetes controller.
- A SIEM replacement.
- A generic shell wrapper.

## Core Concepts

- ToolInvocation: normalized request (`actor`, `tool`, `operation`, `params`, `context`).
- Registry: only known tools and operations are executable.
- Policy: OPA returns `allow`, `risk_level`, `reason`.
- Evidence: segmented store with sealed segments and hash-chain validation.

## Quickstart

Full guide: `docs/QUICKSTART.md`

```bash
go build ./...
EVIDRA_PROFILE=ops EVIDRA_PACKS_DIR=./packs/_core/ops go run ./cmd/evidra-mcp

go run ./cmd/evidra-policy-sim --policy ./policy/policy.rego --input ./examples/invocations/allowed_kubectl_get_dev.json --data ./policy/data.json
go run ./cmd/evidra-evidence verify --evidence ./data/evidence
```

## Ops Packs Included

The ops profile uses intentionally minimal declarative tool surfaces:

- `packs/_core/ops/argocd-basic`
- `packs/_core/ops/terraform-basic`
- `packs/_core/ops/helm-basic`
- `packs/_core/ops/kubectl-basic` (if present in your checkout)

## Evidence Utilities

Use `evidra-evidence` for local forensics:

- `verify` - validate integrity.
- `violations` - summarize denies/high-risk actions.
- `export` - create an audit pack.

## Runtime Profiles

- `ops` (default): production-focused, excludes dev/demo tools by default.
- `dev`: enables dev/demo tool registration for local development.

Examples:

```bash
EVIDRA_PROFILE=ops ./evidra-mcp
EVIDRA_PROFILE=dev ./evidra-mcp
```

### Enforcement Model

Evidra enforces guardrails for MCP tool invocations. To prevent bypass, configure agents in Guarded Mode (no direct shell access, MCP tools via Evidra only). See `docs/MCP_GUIDE.md`.

## Extension Model

- Primary in v0.1: Level 1 declarative Tool Packs (`EVIDRA_PACKS_DIR`).
- Experimental/future: Level 2 compile-time plugins.

## Documentation

- `docs/QUICKSTART.md`
- `docs/DEMO.md`
- `docs/RELEASE_CHECKLIST.md`
- `docs/POLICY_GUIDE.md`
- `docs/TOOL_PACKS.md`
- `docs/OPS_PROFILE.md`
- `docs/EVIDENCE_GUIDE.md`
- `docs/MCP_GUIDE.md`
- `docs/FAQ.md`
- `spec/ARCHITECTURE_COMPONENTS.md`
