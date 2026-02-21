# Evidra

Let AI propose changes. Evidra decides.

Evidra is a monorepo with a shared core policy/evidence runtime and bundle-specific validation flows. The primary entry point today is `evidra ops validate`, which checks infra scenarios before execution and writes immutable evidence.

## 5-Minute Demo

Requirements: Go 1.22+ (recommended 1.23)

```bash
go build ./cmd/evidra

# PASS example
./evidra ops validate ./bundles/ops/examples/scenario_breakglass_audited.json

# FAIL example
./evidra ops validate ./bundles/ops/examples/scenario_s3_public_fail.json
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

## Monorepo Layers

- `core/`: narrative-neutral policy runtime, evaluator interfaces, registry, and evidence primitives.
- `bundles/ops/`: AI-first scenario validation flow for infrastructure changes.
- `bundles/regulated/`: controlled environment validation flow for compliance-oriented operations.

## Where To Start

- Ops bundle docs: `bundles/ops/README.md`
- Regulated bundle docs: `bundles/regulated/README.md`

## CLI Overview

```text
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

## Architecture Notes (Concise)

- Validation decisions are policy-driven.
- Evidence records are append-only and hash-linked.
- Bundles can depend on `core`, but `core` does not depend on bundles.
- Bundle policies and examples live with each bundle to keep intent explicit.
