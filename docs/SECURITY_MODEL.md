# Evidra Security Model

## Enforcement Assumptions

- All validation flows (CLI or MCP) share a single core (`pkg/validate`) that loads the OPA bundle (default `policy/bundles/ops-v0.1`), evaluates the request, and writes an evidence record. Keeping requests inside this control path ensures decisions, hits, and hints stay deterministic.
- The MCP server exposes only the `validate` tool and `get_event` evidence lookup, so registries/execute paths are not part of the v0.1 scope.
- Policy decisions are always recorded, even in `--observe` mode; advisory runs still append evidence with advisory metadata.

## Tamper-Evident Scope

- The evidence store at `~/.evidra/evidence` is append-only and hash-chained. Each written record includes `previous_hash` plus a self-verifying `hash`, making tampering detectable.
- Evidence records contain actor/tool/operation info, policy decision (allow/hints/reasons), and execution metadata (status, optional exit code). The hash covers the canonical JSON representation except for the `hash` field itself.
- If evidence cannot be written, the validation pipeline reports an internal failure so the caller can’t bypass logging.

## Known Bypass Vectors

- Any path that skips `pkg/validate`/`evidra-mcp` or writes to the evidence log directly is out of scope for v0.1.
- Adversaries with root on the host could still rewrite evidence storage unless the evidence directory is mounted on immutable media or exported elsewhere.

## Recommended Deployment Pattern

- Run `evidra-mcp` inside an isolated runtime with network controls so only trusted clients can submit ToolInvocations.
- Limit agent shells/other runtimes so they cannot bypass the MCP server or the offline `evidra validate` CLI.
- Export evidence segments (`evidra evidence export`) to a hardened vault for long-term auditing and independent validation.
