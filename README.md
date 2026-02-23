# Evidra v1-slim

Evidra  deterministically enforces policy on Terraform plans and Kubernetes changes through a local MCP server plus focused offline tooling.

## Features
### MCP server (evidra-mcp)
- Enforces the structured policy under `policy/profiles/ops-v0.1` via OPA and records every decision as an immutable evidence record.
- Supports `--observe` for advisory runs while still logging policy hits, hints, and evidence identifiers.
- Optional `--packs-dir` can load additional tool metadata to shape validations.
- Evidence defaults to `~/.evidra/evidence`; override with `--evidence-dir` or `EVIDRA_EVIDENCE_DIR` (legacy `EVIDRA_EVIDENCE_PATH`) to keep compliance artifacts elsewhere.

### Offline CLI (evidra)
- `evidra validate <file>` auto-detects Terraform plan JSON or Kubernetes manifests and prints PASS/FAIL along with rule IDs, hints, and evidence IDs.
- `--explain` surfaces matching rule labels, reasons, and key action facts; `--json` emits a structured decision payload for automation.
- `evidra policy sim` lets you exercise or debug policy decisions locally.
- `evidra evidence inspect` / `evidra evidence report` explore the hash-linked evidence store.

## Start the MCP server (primary entry point)

```bash
evidra-mcp \
  --policy policy/profiles/ops-v0.1/policy.rego \
  --data   policy/profiles/ops-v0.1/data.json \
  --evidence-dir ~/.evidra/evidence \
  [--packs-dir ./packs/_core/ops] \
  [--observe]
```

- Both `--policy` and `--data` are required (or set `EVIDRA_POLICY_PATH` / `EVIDRA_DATA_PATH`).
- Evidence defaults to `~/.evidra/evidence`, or override with `--evidence-dir` / `EVIDRA_EVIDENCE_DIR` (legacy `EVIDRA_EVIDENCE_PATH`).
- Pass `--observe` to collect advisory output while still evaluating policy decisions and evidence.
- Connect your MCP client (Codex, scripts, etc.) to submit ToolInvocations through the registry → policy → evidence pipeline.

## Offline tools (`evidra`)

```bash
evidra validate examples/terraform_plan_pass.json
```

Add `--explain` to see hits, hints, and action facts, or `--json` to produce a structured response for downstream tooling.

```bash
evidra policy sim --policy policy/profiles/ops-v0.1/policy.rego --input examples/terraform_mass_delete_fail.json
```

```bash
evidra evidence inspect --evidence-dir ~/.evidra/evidence
```

## Policy & evidence

- Policy rules live under `policy/profiles/ops-v0.1/policy/`; rewrite decisions in small deny/warn files and keep hints in `data.json`.
- Evidence records live under `~/.evidra/evidence` (or your configured path); see `docs/evidence.md` for the JSONL schema and inspection commands.
- `docs/policy.md` explains the policy contract and how to run `opa test` against the profile.
- `docs/advanced.md` covers MCP server configuration, registry/packs, and advanced workflows.

## Not in v1
- No regulated bundle or multiple policy profiles—`ops-v0.1` is the single source of truth.
- No hosted MCP service; everything runs locally via `evidra-mcp` or the offline CLI.
- No demo executors or shell-level command runners baked into the registry.
- No `--log-level` / `--listen` flags or hidden configuration knobs (only the flags listed above).
- No features that require runtime downloads or additional dependencies.
