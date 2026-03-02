# Evidra Engine Logic v2 (MCP summary)

- `actor.type` is the security classifier (`human|agent|ci`).
- `actor.origin` is transport metadata (`mcp|cli|api`) and not a security classifier.
- `context.source` is optional metadata and not a security classifier.
- Canonicalization normalizes native and flat Kubernetes payloads before policy rules run.
- If MCP schema validation fails, JSON-RPC returns `-32602` and tool handlers are not invoked.

See repo docs: `docs/ENGINE_LOGIC_V2.md`.
