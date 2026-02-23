# Architecture Components v0.1

Related docs:
- `docs/QUICKSTART.md`
- `docs/DEMO.md`
- `docs/POLICY_GUIDE.md`
- `docs/TOOL_PACKS.md`
- `docs/EVIDENCE_GUIDE.md`
- `docs/RELEASE_CHECKLIST.md`

Evidra v0.1 has three core components:
- Registry: static tool surface and operation validation.
- Policy (OPA): deterministic allow/deny evaluation for ToolInvocation input.
- Evidence: append-only, hash-chained audit log for every attempt.

## Core Interfaces
- `core.PolicySource`: loads policy/data bytes and exposes deterministic `PolicyRef()`.
- `core.PolicyEngine`: evaluates canonical `ToolInvocation` into deterministic policy decisions.
- `core.EvidenceStore`: appends records, validates chain, and exposes `LastHash()`.

## Local Deployment
- Policy source: local Rego file.
- Policy engine: local embedded OPA using loaded policy/data bytes.
- Evidence store: local JSONL append-only log.

## Server-Driven Future (High Level)
- PolicySource can be replaced by a remote policy source.
- EvidenceStore can be replaced by a remote evidence backend.
- Registry/Policy/Evidence flow remains the same; only source/store implementations change.
