# Protocol Errors

## Error Taxonomy

### Level 1: JSON-RPC Transport Errors
- Code `-32602` (Invalid params)
- MCP schema validation failed before handler runs
- Source: MCP SDK
- Consequence: tool-level response objects (`ok`, `policy`, `error`) are not produced for this failure path.

### Level 2: Tool-Level Errors
- In `ValidateOutput.Error` field
- Schema passed; business logic error

| Code | When | Action |
|---|---|---|
| invalid_input | Malformed invocation | Fix input, retry |
| policy_failure | OPA evaluation failed | Check policy bundle |
| evidence_write_failed | Could not write evidence | Check evidence path |
| evidence_chain_invalid | Tamper detection | Investigate evidence |
| not_found | Event ID not in store | Check event_id |
| api_unreachable | Hosted API down | Retry or fall back |
| internal_error | Unexpected failure | Report bug |

### Level 3: Behavior Gate Errors
- In `ValidateOutput.Error` field
- Policy was NOT evaluated; gate blocked first

| Code | When | Action |
|---|---|---|
| stop_after_deny | Same intent already denied | Change plan or escalate |

Behavior gates apply only to actor.type == "agent" or "ci".
Human actors are never gated.
