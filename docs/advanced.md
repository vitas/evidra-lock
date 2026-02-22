# Advanced topics

## MCP server & registry

- The MCP server (`evidra-mcp`) exposes the `execute` tool, accepts canonical `ToolInvocation` payloads, and enforces the registry → policy → evidence flow.
- Required flags: `--policy`, `--data`, `--evidence-dir`. Environment variables `EVIDRA_POLICY_PATH`, `EVIDRA_DATA_PATH`, `EVIDRA_PACKS_DIR`, and `EVIDRA_EVIDENCE_DIR` (fallback `EVIDRA_EVIDENCE_PATH`) can override them.
- Use `--packs-dir` or `EVIDRA_PACKS_DIR` to load tool packs such as `packs/_core/ops`. Set `--observe` to collect advisory evidence without blocking execution.
- `EVIDRA_MODE=enforce|observe` still overrides enforcement mode when you prefer env config over flags.
- Registry/packs supply tool metadata; see `packs/_core/ops` for the canonical definitions.

## Offline tools

- `evidra validate <file>` runs the structured validator.
- `evidra policy sim --policy <path> --input <path> [--data <path>]` evaluates policy decisions locally.
- `evidra evidence <verify|export|violations|cursor>` inspects the append-only evidence store and exports audit packs.

## When to dive deeper

- Use `docs/policy.md` to understand rule files, add new denies/warns, and run `opa test`.
- Use `docs/evidence.md` to inspect the JSONL store, understand policy decisions in each record, and guard evidence integrity.
