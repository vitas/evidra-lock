# Evidra v1-slim

Evidra v1-slim deterministically enforces policy on Terraform plans and Kubernetes changes via the local MCP server and offline tooling.

## Start the MCP server (primary entry point)

```bash
evidra-mcp \
  --policy policy/profiles/ops-v0.1/policy.rego \
  --data   policy/profiles/ops-v0.1/data.json \
  --evidence-dir ~/.evidra/evidence \
  [--packs-dir ./packs/_core/ops] \
  [--log-level info] \
  [--observe]
```

- Both `--policy` and `--data` are required (environment variables `EVIDRA_POLICY_PATH`/`EVIDRA_DATA_PATH` can also supply them).
- Evidence defaults to `~/.evidra/evidence`, or you can override it with `--evidence-dir` / `EVIDRA_EVIDENCE_PATH`.
- Pass `--observe` when you only want advisory evidence while allowing execution.
- Connect your MCP client (Codex or scripts) to submit ToolInvocations through the registry → policy → evidence pipeline.

## Offline tools (`evidra`)

Use the offline CLI to inspect policy decisions and evidence without running the server:

```bash
evidra validate examples/terraform_plan_pass.json
evidra policy sim --policy policy/profiles/ops-v0.1/policy.rego --input examples/terraform_mass_delete_fail.json
evidra evidence inspect --evidence-dir ~/.evidra/evidence
```

Add `--explain` to `evidra validate` to print rule IDs, hints, and action facts for each deny.

## Policy & evidence

- Policy rules live under `policy/profiles/ops-v0.1/policy/`.
- Evidence is recorded under `~/.evidra/evidence` (or your configured path); see `docs/evidence.md` for the JSONL schema.

## Advanced topics

- `docs/advanced.md` walks through the MCP server flags, registry/packs, and offline helpers.
- `docs/policy.md` explains the rule structure plus `opa test`.
- `docs/evidence.md` describes the evidence store and inspection workflows.
