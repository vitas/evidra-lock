# Protocol Errors

## `-32602` Invalid params

- Means request arguments failed MCP schema validation.
- In this path, tool handlers are not invoked.
- Tool-level fields like `ok`, `policy`, or `error` are not produced.

See repo docs: `docs/PROTOCOL_ERRORS.md`.
