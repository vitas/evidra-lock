# Protocol Errors

Short reference for MCP/JSON-RPC error codes that appear in Evidra integration tests.

## `-32602` — Invalid params

- Meaning: request arguments failed schema/parameter validation.
- In Evidra MCP this typically means `validate` arguments did not satisfy `validate.schema.json` (for example invalid `actor.origin` enum).
- Execution model: schema validation happens before tool handler execution.
- Consequence: tool-level response objects (for example `ok`, `policy`, `error`) are not produced for this failure path.
