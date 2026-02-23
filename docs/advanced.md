# Advanced topics

-## MCP server & evidence

- The MCP server (`evidra-mcp`) exposes the `validate` tool (and the `get_event` resource) to enforce the policy → evidence flow for every `ToolInvocation`.
- Both MCP (`evidra-mcp`) and offline CLI (`evidra validate`) now route through the shared `pkg/validate` core that uses `pkg/scenario` for scenario loading, so the decision output (hits/hints/reasons) and evidence chain stay identical.
- Required flags: `--policy`, `--data`, and optional `--evidence-store` (alias `--evidence-dir`). Environment variables `EVIDRA_POLICY_PATH`, `EVIDRA_DATA_PATH`, and `EVIDRA_EVIDENCE_DIR` (legacy `EVIDRA_EVIDENCE_PATH`) override them when needed.
- Default evidence store path is always `~/.evidra/evidence`.
- Set `--observe` or `EVIDRA_MODE=observe` to capture advisory evidence while still recording denials; the validation layer still enforces tool schemas.
- The MCP server responds with hits/hints, risk_level, and evidence IDs so clients can surface structured decision summaries and link back to the immutable evidence store.

## Offline tools

- `evidra validate <file>` runs the structured validator.
- `evidra policy sim --policy <path> --input <path> [--data <path>]` evaluates policy decisions locally.
- `evidra evidence <verify|export|violations|cursor>` inspects the append-only evidence store and exports audit artifacts.

## When to dive deeper

- Use `docs/policy.md` to understand rule files, add new denies/warns, and run `opa test`.
- Use `docs/evidence.md` to inspect the JSONL store, understand policy decisions in each record, and guard evidence integrity.
