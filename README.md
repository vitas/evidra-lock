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

## Evidra Evidence Utilities

Verify evidence hash-chain integrity:

```bash
go run ./cmd/evidra-evidence verify --evidence ./data/evidence
```

Export an audit pack:

```bash
go run ./cmd/evidra-evidence export --evidence ./data/evidence --out ./audit-pack.tar.gz --policy ./policy/policy.rego --data ./policy/data.json
```

Generate a violations summary report:

```bash
go run ./cmd/evidra-evidence violations --evidence ./data/evidence --min-risk high
```

Show cursor bookmark:

```bash
go run ./cmd/evidra-evidence cursor show --evidence ./data/evidence
```

Acknowledge cursor position:

```bash
go run ./cmd/evidra-evidence cursor ack --evidence ./data/evidence --segment evidence-000001.jsonl --line 0
```

Audit pack contents:
- Segmented evidence store files: `evidence/manifest.json` and `evidence/segments/evidence-*.jsonl` (or legacy `evidence/evidence.log`).
- `manifest.json` with record count, last hash, and policy reference.
- Optional `policy/policy.rego` and `policy/data.json` snapshots.

Violations report:
- Includes denied actions and high-risk actions that meet `--min-risk`.
- Requires a valid evidence hash-chain before reporting.
- Cursor is a local bookmark for future forwarders/exporters.

Default evidence store layout:
- Root: `./data/evidence`
- Manifest: `./data/evidence/manifest.json`
- Segments: `./data/evidence/segments/evidence-000001.jsonl`, `evidence-000002.jsonl`, ...
- Segment size env: `EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES` (default `5000000`)
- Sealed segments: manifest field `sealed_segments` tracks completed immutable segments.
- Rotation seals the previous `current_segment` and advances to the next segment file.
- Sealed segments provide stable units for local forward/export workflows.

## Local Workflow (Policy + Evidence)

Offline policy tests:

```bash
make policy-sim-echo
make policy-sim-kubectl-deny
```

Start MCP server locally:

```bash
make run-mcp
```

- MCP is the primary integration path for agents.
- `policy-sim` is for local policy iteration without an agent.
- `evidra-evidence` is for verification, export, and violations forensics.

Evidence forensics:

```bash
make evidence-verify
make evidence-violations
make evidence-export
```

## Kubernetes Dev Notes (Optional)

- This repo does not manage clusters.
- To test kubectl-oriented invocations, use a local cluster (kind or minikube).
- Set environment in `invocation.context` (for example `dev` or `prod`).

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

MCP tools:
- `execute`: runs a registered tool invocation through registry, policy, and evidence.
- `get_event`: fetches one immutable evidence record by `event_id` (chain-safe read).

Tool surface extension:
- Primary (v0.1): declarative Tool Packs (Level 1) loaded from `EVIDRA_PACKS_DIR` for local extension without new binaries.
- Experimental / future: compile-time plugins (Level 2), registered explicitly in `cmd/evidra-mcp`.
- `kubectl` is currently provided in this repository as an experimental compile-time plugin.

Example flow:
1. Call `execute` and capture returned `event_id`.
2. Call `get_event` with `{"event_id":"<returned_event_id>"}` to retrieve the full record.

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
