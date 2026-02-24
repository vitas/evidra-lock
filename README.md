# Evidra v0.1

Evidra deterministically enforces policy on infrastructure changes through a local MCP server plus focused offline tooling.

## Features
### MCP server (evidra-mcp)
- Enforces the OPA bundle policy under `policy/bundles/ops-v0.1` and records every decision as an immutable evidence record.
- Exposes the `validate` tool (and `get_event` for fetching evidence) so MCP clients can submit ToolInvocations, review hits/hints, and link decisions to evidence.
- Supports `--observe` for advisory runs while still logging policy hits, hints, and evidence identifiers.
- Evidence defaults to `~/.evidra/evidence`; override with `--evidence-store` (or `--evidence-dir`) or `EVIDRA_EVIDENCE_DIR`.

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
  --bundle policy/bundles/ops-v0.1 \
  --evidence-store ~/.evidra/evidence \
  [--observe]
```

- Pass `--bundle` to point at an OPA bundle directory (or set `EVIDRA_BUNDLE_PATH`).
- Pass `--environment` to set the environment label for policy evaluation (or set `EVIDRA_ENVIRONMENT`).
- Evidence defaults to `~/.evidra/evidence`, or override with `--evidence-store` (or `--evidence-dir`) / `EVIDRA_EVIDENCE_DIR`.
- Pass `--observe` to collect advisory output while still evaluating policy decisions and evidence.
- Connect your MCP client (Codex, scripts, etc.) to submit ToolInvocations through the validate → policy → evidence pipeline.

## Offline tools (`evidra`)

```bash
evidra validate examples/terraform_plan_pass.json
```

Add `--explain` to see hits, hints, and action facts, or `--json` to produce a structured response for downstream tooling.

```bash
evidra policy sim --policy policy/bundles/ops-v0.1/evidra/policy/policy.rego --input examples/terraform_mass_delete_fail.json
```

Use `EVIDRA_EVIDENCE_DIR` to override the evidence store path for offline commands.

## Policy & evidence

- Policy lives in OPA bundle format under `policy/bundles/ops-v0.1/`; rules are in `evidra/policy/rules/`, data-driven params in `evidra/data/params/data.json`, and hints in `evidra/data/rule_hints/data.json`.
- Rule IDs use canonical `domain.invariant_name` format (e.g., `k8s.protected_namespace`, `ops.mass_delete`). See `ai/AI_RULE_ID_NAMING_STANDARD.md` for the naming convention.
- Evidence records live under `~/.evidra/evidence` (or your configured path); see `docs/evidence.md` for the JSONL schema and inspection commands.
- `docs/policy.md` explains the policy contract and how to run `opa test` against the bundle.
- `docs/advanced.md` covers MCP server configuration and advanced workflows.
- `docs/mcp-clients-setup.md` contains client integration setup (Codex, Gemini, Claude Desktop).
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
- No external pack/registry onboarding; policy edits stay under `policy/bundles/ops-v0.1`.
- No hosted MCP service or runtime downloads; everything runs locally via `evidra-mcp` or `evidra`.
