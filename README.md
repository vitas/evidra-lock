# Evidra v1-slim

Evidra deterministically enforces policy on infrastructure changes through a local MCP server plus focused offline tooling.

## Features
### MCP server (evidra-mcp)
- Enforces the structured policy under `policy/profiles/ops-v0.1` via OPA and records every decision as an immutable evidence record.
- Exposes the `validate` tool (and `get_event` for fetching evidence) so MCP clients can submit ToolInvocations, review hits/hints, and link decisions to evidence.
- Supports `--observe` for advisory runs while still logging policy hits, hints, and evidence identifiers.
- Evidence defaults to `~/.evidra/evidence`; override with `--evidence-store` (or `--evidence-dir`) or `EVIDRA_EVIDENCE_DIR` (legacy `EVIDRA_EVIDENCE_PATH`).

### Shared evaluation core
- Both `evidra` and `evidra-mcp` rely on the same Go evaluation core (`pkg/validate`) that uses `pkg/scenario` for input loading, runs policy via `pkg/runtime`/`pkg/policy`, and records every decision through `pkg/evidence`. This pipeline preserves identical decision output, hits, hints, and evidence IDs regardless of entry point.

### Offline CLI (evidra)
- `evidra validate <file>` auto-detects structured plan/manifest files—examples include Terraform plan JSON and Kubernetes manifests—and prints PASS/FAIL along with rule IDs, hints, and evidence IDs.
- `--explain` surfaces matching rule labels, reasons, and key action facts; `--json` emits a structured decision payload for automation.
- `evidra policy sim` lets you exercise or debug policy decisions locally.
- `evidra evidence inspect` / `evidra evidence report` explore the hash-linked evidence store.

## Start the MCP server (primary entry point)

```bash
evidra-mcp \
  --policy policy/profiles/ops-v0.1/policy.rego \
  --data   policy/profiles/ops-v0.1/data.json \
  --evidence-store ~/.evidra/evidence \
  [--observe]
```

- Both `--policy` and `--data` are required (or set `EVIDRA_POLICY_PATH` / `EVIDRA_DATA_PATH`).
- Evidence defaults to `~/.evidra/evidence`, or override with `--evidence-store` (or `--evidence-dir`) / `EVIDRA_EVIDENCE_DIR` (legacy `EVIDRA_EVIDENCE_PATH`).
- Pass `--observe` to collect advisory output while still evaluating policy decisions and evidence.
- Connect your MCP client (Codex, scripts, etc.) to submit ToolInvocations through the validate → policy → evidence pipeline.

## Offline tools (`evidra`)

```bash
evidra validate examples/terraform_plan_pass.json
```

Add `--explain` to see hits, hints, and action facts, or `--json` to produce a structured response for downstream tooling.

```bash
evidra policy sim --policy policy/profiles/ops-v0.1/policy.rego --input examples/terraform_mass_delete_fail.json
```

Use `EVIDRA_EVIDENCE_DIR` (or legacy `EVIDRA_EVIDENCE_PATH`) to override the evidence store path for offline commands.

## Policy & evidence

- Policy rules live under `policy/profiles/ops-v0.1/policy/`; rewrite decisions in small deny/warn files and keep hints in `data.json`.
- Evidence records live under `~/.evidra/evidence` (or your configured path); see `docs/evidence.md` for the JSONL schema and inspection commands.
- `docs/policy.md` explains the policy contract and how to run `opa test` against the profile.
- `docs/advanced.md` covers MCP server configuration and advanced workflows.
- `docs/architecture.md` summarizes the v0.1 runtime graph from Codex through MCP to policy/evidence.

## Features (v0.1)

- MCP `validate` tool plus `get_event` evidence lookup.
- Policy decisions always include hits, hints, and reasons for any deny.
- Immutable evidence chain stored under `~/.evidra/evidence`.
- Offline `evidra validate` auto-detects plan or manifest inputs and prints structured summaries.
- Evidence inspection/reporting via `evidra evidence inspect`/`report`.

## Architecture (v0.1)

- See `docs/architecture.md` for the call graph from Codex through `evidra-mcp`, `pkg/validate`, `pkg/runtime`, and `pkg/evidence`.

## Not in v0.1

- `execute` tool, registry packs, and plugin-based executors are not part of v0.1.
- No external pack/registry onboarding; policy/profile edits stay under `policy/profiles/ops-v0.1`.
- No hosted MCP service or runtime downloads; everything runs locally via `evidra-mcp` or `evidra`.
