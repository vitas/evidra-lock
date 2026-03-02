# MCP Guidance Content

This directory is the runtime source of truth for MCP guidance text.

You can tune these files without recompiling `evidra-mcp`:

- `initialize/instructions.txt` — Initialize instructions returned by MCP server.
- `tools/*.txt` — tool descriptions (`validate`, `get_event`).
- `resources/content/*.md` — resource payloads served by MCP.
- `resources/descriptions/*.txt` — resource descriptions shown in metadata.

Resolution order at runtime:

1. `--content-dir`
2. `EVIDRA_CONTENT_DIR`
3. Auto-discovery of `prompts/mcpserver` by walking parent directories

If required files are missing, server startup fails fast.
