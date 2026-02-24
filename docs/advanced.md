# Advanced topics

## MCP server & evidence

- The MCP server (`evidra-mcp`) exposes the `validate` tool (and the `get_event` resource) to enforce the policy → evidence flow for every `ToolInvocation`.
- Both MCP (`evidra-mcp`) and offline CLI (`evidra validate`) route through the shared `pkg/validate` core that uses `pkg/scenario` for scenario loading, so the decision output (hits/hints/reasons) and evidence chain stay identical.
- Required flag: `--bundle` pointing to an OPA bundle directory (e.g., `policy/bundles/ops-v0.1`). Environment variables `EVIDRA_BUNDLE_PATH` and `EVIDRA_EVIDENCE_DIR` override flags when needed.
- Pass `--environment` (or set `EVIDRA_ENVIRONMENT`) to set the environment label for data-driven param resolution in policy rules.
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
