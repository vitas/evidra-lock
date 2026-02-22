# Advanced topics

## MCP server & registry

- The MCP server exposes the `execute` tool, accepts canonical `ToolInvocation` payloads, and enforces the registry → policy → evidence flow.
- Environment flags:
  - `EVIDRA_MODE=enforce|observe` (default enforce, observe records advisory evidence without blocking).
  - `EVIDRA_POLICY_PATH`, `EVIDRA_DATA_PATH`, `EVIDRA_PACKS_DIR`, and `EVIDRA_EVIDENCE_PATH` override defaults.
- Registry/packs supply tool metadata; see `packs/_core/ops` for the canonical definitions.

## Auxiliary commands

- `evidra ops init` bootstraps `.evidra/ops.yaml` and validator samples (advanced use only).
- `evidra ops explain <schema|kinds|example|policies>` prints schema/policy guidance based on the ops bundle.
- `evidra policy sim` evaluates the policy offline (`--policy`, `--data`, `--input` flags).
- `evidra evidence <verify|export|violations|cursor>` inspects the append-only evidence store and exports audit packs.

## When to dive deeper

- Use `docs/policy.md` to understand rule files, add new denies/warns, and run `opa test`.
- Use `docs/evidence.md` to inspect the JSONL store, understand policy decisions in each record, and guard evidence integrity.
