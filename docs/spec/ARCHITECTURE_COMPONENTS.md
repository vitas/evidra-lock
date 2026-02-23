# Architecture Components v0.1

Related docs:
- `docs/QUICKSTART.md`
- `docs/DEMO.md`
- `docs/POLICY_GUIDE.md`
- `docs/TOOL_PACKS.md`
- `docs/EVIDENCE_GUIDE.md`
- `docs/RELEASE_CHECKLIST.md`

Evidra v0.1 has three core responsibilities:
- `pkg/mcpserver`: adapters (CLI + MCP) that accept ToolInvocation-like input and forward it to the shared pipeline.
- `pkg/validate`: shared core that consolidates scenario loading (`pkg/scenario`), policy evaluation (`pkg/runtime` + `policy`), and evidence recording (`pkg/evidence`).
- `pkg/evidence`: append-only, hash-chained audit log for every validation attempt.

## Local Deployment
- CLI/MCP adapters load files or invocations.
- `pkg/validate` loads the policy profile `policy/profiles/ops-v0.1`, evaluates it with OPA, and records hits/hints into evidence.
- `pkg/evidence` persists JSONL segments plus resource link manifests.

## Server-Driven Future (High Level)
- Adapters remain the entrypoints.
- Policy modules can be swapped via `--policy`/`--data` without touching the Go pipeline.
- Evidence can be exported or forwarded once generated.
