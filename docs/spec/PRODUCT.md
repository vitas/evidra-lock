# Evidra

Evidra is a controlled execution core that runs registered tools under policy and records immutable evidence.

## Principles
- Controlled tool surface (registered tools only)
- Policy enforcement using OPA/Rego (default deny)
- Immutable evidence ledger (append-only, hash-chained)
- Transport-agnostic core (MCP/CLI are adapters)
- Minimal surface area (no generic shell)
