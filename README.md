# Evidra

**Policy-Enforced AI Tool Execution with Audit-Ready Evidence**

Let AI propose changes. Evidra decides. Two deployment models:

- 🔍 **AI Governance & Validation Mode**
- 🛡 **Regulated Enforcement Mode**

---

# 🔍 AI Governance & Validation Mode

Designed for platform teams and AI infrastructure engineers.

## Goal

Make AI tool execution observable, measurable, and tunable.

## Core Principles

- Observe mode (execution allowed with advisory evidence)  
- Risk scoring and violation filtering  
- Policy simulation  
- Tool pack validation  
- Execution transparency  

## Typical Use Cases

- Internal AI tooling  
- Safety boundary testing  
- Policy tuning before enforcement  
- ML platform governance  

---

# 🛡 Regulated Enforcement Mode

Designed for financial services, healthcare, public sector, and compliance-heavy SaaS platforms.

## Goal

Prevent unauthorized or unsafe AI automation before it happens.

## Core Principles

- Default-deny execution  
- Registered tools only  
- Policy evaluation before execution  
- Guarded Mode (no direct shell bypass)  
- Tamper-evident evidence records  
- Exportable audit artifacts  

## Typical Use Cases

- AI agents operating in production  
- Controlled Kubernetes operations  
- Compliance audit workflows  
- Incident-ready evidence export  

---

# Quick Start

See:

- docs/QUICKSTART.md  
- docs/POLICY_GUIDE.md  
- docs/EVIDENCE_GUIDE.md  

---

# Security Model

See:

- docs/MCP_GUIDE.md  
- spec/SCOPE_AND_ASSUMPTIONS.md  

Let AI propose changes. Evidra decides.

Evidra is a monorepo with a shared core policy/evidence runtime and bundle-specific validation flows. The primary entry point today is `evidra ops validate`, which checks infra scenarios before execution and writes immutable evidence.


## Guarded Mode

Guarded Mode enables strict gateway enforcement for MCP execution:

- only registered tools/operations may run,
- bypass-style invocations (shell/script/binary-path patterns) are denied,
- denials are recorded in evidence with explicit reason codes.

Use Guarded Mode for production and regulated environments:

```bash
go run ./cmd/evidra mcp --guarded
```

Limitations:

- it protects only flows that pass through Evidra-MCP,
- direct host execution channels outside the gateway are out of scope.

## 5-Minute Demo

Requirements: Go 1.22+ (recommended 1.23)

```bash
go build ./cmd/evidra

# Bootstrap local config + starter examples
./evidra ops init

# PASS example
./evidra ops validate ./.evidra/examples/scenario_breakglass_audited.json

# FAIL example
./evidra ops validate ./.evidra/examples/scenario_kubectl_apply_prod_block.json
```

Expected output shape:

```text
Decision: PASS
Risk level: high
Evidence: evt-...
Reason: ...
```

```text
Decision: FAIL
Risk level: high
Evidence: evt-...
Reason: ...
```

## Where To Start

- Ops bundle docs: `bundles/ops/README.md`
- Regulated bundle docs: `bundles/regulated/README.md`

## CLI Overview

```text
evidra mcp [--guarded] [--policy path] [--data path]
evidra policy sim --policy <path> --input <path> [--data <path>]
evidra evidence <verify|export|violations|cursor> ...
evidra ops init [--path dir] [--force] [--enable-validators] [--with-plugins] [--minimal] [--print]
evidra ops validate <file>
evidra ops explain schema|kinds|example|policies [--verbose]
evidra regulated validate <file>
```

## Repository Documentation

- `docs/INDEX.md`
- `docs/QUICKSTART.md`
- `docs/DEMO.md`
- `docs/POLICY_GUIDE.md`
- `docs/TOOL_PACKS.md`
- `docs/EVIDENCE_GUIDE.md`
- `docs/MCP_GUIDE.md`

