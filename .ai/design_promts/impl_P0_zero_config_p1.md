You are a senior Go engineer.

Task:
Implement zero-config embedded OPA bundle for evidra-mcp.

Goal:
`evidra-mcp` must start with zero flags on a clean machine.

Requirements:

- Use //go:embed to embed policy/bundles/ops-v0.1
- If --bundle is not provided and EVIDRA_BUNDLE_PATH is not set:
  - Extract embedded bundle to a temporary directory
  - Use that directory as bundle path
- Print to stderr:
  "using built-in ops-v0.1 bundle"

Constraints:

- No changes to CLI behavior
- No breaking existing --bundle flag
- No new dependencies
- Keep changes limited to cmd/evidra-mcp and bundle resolution logic

Deliver:
- Modified main.go
- Helper extraction function
- Any required changes in pkg/config
- Minimal, clean code