# AI Decisions Log

## Purpose
Record all architectural and refactoring decisions influenced by AI, including rationale and impact.

## 2026-02-19 - Samebits Core v0.1 Evidence Ledger
- Decision: Use a generic `EvidenceRecord` schema with neutral fields (`id`, `timestamp`, `actor`, `action`, `subject`, `details`, `prev_hash`, `hash`) because the provided schema placeholder was not populated.
- Rationale: Preserve domain neutrality while providing enough structure for immutable chain linking and audit replay.
- Decision: Compute record hash from canonical JSON that excludes `hash` by marshaling a dedicated canonical struct.
- Rationale: Enforces stable hashing semantics and prevents self-referential hash inclusion.
- Decision: Persist records as append-only JSONL at `./data/evidence.log`; set `prev_hash` to prior record hash during append.
- Rationale: Aligns with immutable ledger requirements and enables sequential chain validation.
- Decision: Validate chain by recomputing each hash and verifying `prev_hash` linkage from head to tail.
- Rationale: Detects both content tampering and link tampering with minimal complexity.

## 2026-02-20 - Evidra MCP v0.1 Layered Execution Flow
- Decision: Use official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) with `mcp.NewServer`, `mcp.AddTool`, and stdio transport (`mcp.StdioTransport`).
- Rationale: Satisfies official SDK requirement while keeping transport minimal.
- Decision: Introduce `pkg/mcpserver` as a transport adapter with thin handler delegating to `ExecuteService`.
- Rationale: Enforces separation of concerns and keeps business decisions out of MCP handler functions.
- Decision: Introduce `pkg/policy` with default-deny exact command matching loaded from `config/policy.yaml`.
- Rationale: Meets deterministic allow-list requirement with minimal policy surface.
- Decision: Introduce `pkg/executor` wrapper around `exec.CommandContext` with 10-second timeout, no shell invocation, and captured stdout/stderr/exit code.
- Rationale: Ensures safe command execution and deterministic output shape.
- Decision: Add `pkg/evidence.Store` thin abstraction for initialization and append/validate calls.
- Rationale: Allows dependency injection into server layer without embedding evidence logic into handlers.
- Decision: Add dependency `gopkg.in/yaml.v3` to parse policy YAML.
- Rationale: Required to support `config/policy.yaml` loading while maintaining simple, explicit parsing behavior.
