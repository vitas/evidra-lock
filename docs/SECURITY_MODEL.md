# Evidra Security Model

## Enforcement Assumptions

- All tool execution must pass through Evidra-MCP.
- Agents must not have direct shell access.
- Guarded Mode (`--guarded`) is recommended for production.

## Tamper-Evident Scope

- Evidence records are append-only and hash-chained.
- Chain validation detects accidental or partial modification.
- Full host/filesystem compromise is out of scope for v0.1 protections.
- Future roadmap includes off-host anchoring and stronger attestations.

## Known Bypass Vectors

- Direct OS access outside Evidra-MCP.
- Separate execution channels not routed through the gateway.
- Manual evidence rewriting if attacker controls the host and storage.

## Recommended Deployment Pattern

- Run gateway in an isolated container or dedicated runtime boundary.
- Remove shell tools from agent containers/sandboxes.
- Use network isolation so execution pathways are explicit.
- Export evidence off-host for durable audit and forensic retention.
