# Evidra Local Skill Integration Research (CodeX)

## 1) Executive Summary (Comparison Table)

| Platform | Supports custom tools? | Requires JSON Schema? | Supports local tool execution? | Supports external process execution? | Tool declaration format | Invocation model | Notes |
|---|---|---|---|---|---|---|---|
| OpenAI (GPT tools / function calling) | **Confirmed** | **Confirmed** (`function.parameters` JSON Schema object) | **Partially supported** (tool execution is implemented by your app) | **Partially supported** (you can call local binaries from your app runtime) | `tools: [{type:"function", function:{name, description, parameters, strict?}}]` | Model-initiated function call + client executes + client returns `function_call_output` | `tool_choice` supported; strict mode has additional constraints ([OpenAI Function Calling](https://platform.openai.com/docs/guides/function-calling), [OpenAI Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)) |
| Anthropic Claude (tool use) | **Confirmed** | **Confirmed** (`input_schema`) | **Partially supported** (run tool in client code) | **Partially supported** (client may call local processes) | `tools: [{name, description, input_schema}]` | Model emits `tool_use` block (`stop_reason: "tool_use"`), client returns `tool_result` | `tool_choice` modes documented; parallel tool use documented ([Anthropic Tool Use Overview](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/overview), [Implement Tool Use](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/implement-tool-use)) |
| Google Gemini (function calling) | **Confirmed** | **Confirmed** (OpenAPI schema subset for declarations) | **Confirmed** for local code execution pattern | **Partially supported** (client code can launch local binaries) | `tools: [{functionDeclarations:[...]}]` | Model returns function call parts; client executes and returns function response parts | Function calling modes `AUTO/ANY/NONE/VALIDATED`; Python/JS automatic function calling called out ([Gemini Function Calling](https://ai.google.dev/gemini-api/docs/function-calling)) |
| OpenAI Codex app/CLI native “skills” config | **Unclear** | **Unverified — no official documentation found** for a stable cross-client JSON tool schema equivalent to API function-calling schema | **Unclear** | **Unclear** | **Unverified — no official documentation found** in retrieved pages | **Unclear** | Codex docs navigation includes Skills/Tools sections, but this research did not find a stable published schema contract in retrieved source pages ([Codex docs index](https://developers.openai.com/codex/)) |

## 2) Tool/Function Calling Specification Comparison

### OpenAI (GPT tools/function calling)
- Terminology: **function calling**, **tools**.
- Definition format: function tool with `name`, `description`, `parameters` (JSON Schema), optional `strict`.
- Invocation: model can decide when to call tools; `tool_choice` can control this.
- Return path: client executes tool/function, then returns tool output via `function_call_output`.
- Schema strictness: strict mode requires extra constraints (all fields required, `additionalProperties: false`).
- Errors: no single mandatory tool-error envelope found; application-level error object is standard practice.
- Limits: tool-specific payload size limits not explicitly documented in reviewed pages.
  - **Unverified — no official documentation found** for tool-call payload hard limits beyond model context/output limits.
- Streaming: streaming is documented for response events.

Official snippet (minimal, doc-aligned):
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "validate_plan",
        "description": "Validate infra change payload",
        "parameters": {
          "type": "object",
          "properties": {
            "payload": { "type": "object" },
            "profile": { "type": "string" }
          },
          "required": ["payload"],
          "additionalProperties": false
        },
        "strict": true
      }
    }
  ]
}
```

Sources:
- [OpenAI Function Calling](https://platform.openai.com/docs/guides/function-calling)
- [OpenAI Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
- [OpenAI Responses Streaming](https://platform.openai.com/docs/guides/streaming-responses)

### Anthropic Claude (tool use)
- Terminology: **tool use**, **tools**, `tool_use` / `tool_result`.
- Definition format: tools with `name`, `description`, `input_schema`.
- Invocation: model emits a `tool_use` content block; response has `stop_reason: "tool_use"`.
- Return path: application executes tool and sends `tool_result`.
- Schema strictness: JSON schema is required for inputs; strict validation semantics are not fully specified.
  - **Partially supported** for strictness guarantees.
- Errors: no single mandatory wire-format for tool errors found in reviewed docs.
  - **Unverified — no official documentation found** for a required universal error object.
- Limits: no tool-specific payload cap found in reviewed docs; token budgets and prompt caching/token-efficient options are documented generally.
- Streaming: fine-grained tool streaming documented.

Official snippet (minimal, doc-aligned):
```json
{
  "tools": [
    {
      "name": "validate_plan",
      "description": "Validate infra change payload",
      "input_schema": {
        "type": "object",
        "properties": {
          "payload": { "type": "object" },
          "profile": { "type": "string" }
        },
        "required": ["payload"]
      }
    }
  ]
}
```

Sources:
- [Anthropic Tool Use Overview](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/overview)
- [Anthropic Implement Tool Use](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/implement-tool-use)
- [Anthropic Fine-grained Tool Streaming](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/fine-grained-tool-streaming)

### Google Gemini (function calling)
- Terminology: **function calling**, **function declarations**.
- Definition format: function declarations in `tools`; schema uses an OpenAPI subset.
- Invocation: model returns function call; application executes and sends function response.
- Schema strictness: behavior depends on mode (`AUTO`, `ANY`, `NONE`, `VALIDATED`); `ANY` and `VALIDATED` increase schema adherence.
- Errors: no universal mandatory error envelope found; function response payload is application-defined.
- Limits: function descriptions count toward input token limits; supported model list documented on page.
- Streaming: general response streaming is documented, but platform-specific function-call streaming guarantees were not explicit in reviewed function-calling page.
  - **Unverified — no official documentation found** in reviewed pages for tool-call streaming guarantees per mode.

Official snippet (minimal, doc-aligned):
```json
{
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "validate_plan",
          "description": "Validate infra change payload",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "payload": { "type": "OBJECT" },
              "profile": { "type": "STRING" }
            },
            "required": ["payload"]
          }
        }
      ]
    }
  ]
}
```

Sources:
- [Gemini Function Calling](https://ai.google.dev/gemini-api/docs/function-calling)
- [Gemini Token Counting](https://ai.google.dev/gemini-api/docs/tokens)

## 3) Platform-by-Platform Deep Dive

### A) OpenAI (Codex / GPT tools)
**Confirmed**
- Tool definition via `tools` + function schema.
- Model signals tool calls; client executes and returns `function_call_output`.
- `tool_choice` supports auto/required/forced behavior.
- Strict mode exists and enforces extra schema constraints.

**Partially supported**
- Local execution is possible through client/runtime wrappers (not a server-side execution feature).

**Unclear / Unverified**
- Codex-specific local “skills” declaration format equivalent to API function schema:
  - **Unverified — no official documentation found** in retrieved Codex pages for a single stable schema contract.

Example exchange (conceptual, API-aligned):
1. Client sends user request + tool schema.
2. Model returns tool call for `validate_plan`.
3. Client runs local `evidra validate --json ...`.
4. Client sends tool result back.
5. Model produces final response.

### B) Anthropic Claude
**Confirmed**
- Tools are declared with `input_schema`.
- Model emits `tool_use`; application executes and returns `tool_result`.
- `tool_choice` supports explicit control.

**Partially supported**
- Strict schema guarantees are less explicit than OpenAI strict mode.

Differences vs OpenAI:
- Message blocks are explicit (`tool_use` / `tool_result`) instead of `function_call_output`.
- Stop reason (`tool_use`) is part of invocation control.

### C) Google Gemini
**Confirmed**
- Function calling supports declarations + invocation + function response cycle.
- Local-code functions are part of documented usage.
- Mode controls (`AUTO`, `ANY`, `NONE`, `VALIDATED`) are documented.

**Partially supported**
- External process invocation is still application responsibility.

**Unclear**
- Uniform strict-schema behavior parity with other providers across all modes.

## 4) Concrete Integration Strategies for Evidra

### A) Direct CLI Execution Strategy
Agent runtime executes local binary and parses JSON.

- Command pattern:
  - `evidra validate --json <file>`
- For inline payloads:
  - write temp file, invoke CLI, parse stdout JSON.

Pros:
- Fastest to ship.
- No extra daemon.
- Works across languages.

Cons:
- Process spawn overhead.
- Needs robust temp-file handling for large payloads.
- `get_evidence` by `evidence_id` needs a stable retrieval command (currently a gap).

Implementation difficulty: **Low**
Portability: **High**

### B) Local Wrapper Service Strategy
Small local service wraps `evidra` CLI/library and exposes `validate_plan`/`get_evidence`.

Pros:
- Stable contract for multiple clients.
- Better batching/caching.

Cons:
- Extra process lifecycle.
- More operational surface than direct CLI.

Implementation difficulty: **Medium**
Portability: **Medium-High**

### C) Embedded Library Strategy
Import Evidra Go packages directly into a Go-based agent runtime.

Pros:
- Lowest latency.
- Strong type safety.

Cons:
- Go-only.
- Tight coupling to internal package contracts.

Implementation difficulty: **Medium**
Portability: **Low**

## 5) Required Changes in Evidra Project

### Mandatory for MVP
1. Freeze a **versioned JSON output contract** for `validate_plan` mapping from `evidra validate --json`.
2. Define deterministic mapping:
   - `decision`: `allow | deny | observe`
   - `violations[]`: `{rule_id, severity, message, hint?}`
   - `evidence_id`
   - `summary`
3. Add a first-class evidence lookup command by ID for `get_evidence` (currently no explicit `get by event_id` CLI surface).
4. Publish provider-specific tool schema files under `examples/skills/`.
5. Add compatibility tests for wrappers (invalid JSON, unknown profile, oversized payload behavior).

### Nice to have
1. Explicit `--output json --schema-version v1` aliases for long-term stability.
2. Exit-code contract doc (`allow=0`, `deny=2`, runtime errors non-zero).
3. Deterministic key ordering tests for machine consumption.
4. Small SDK wrappers (Go/Python/TypeScript) that shell out to CLI.

## 6) JSON Schema Normalization Strategy

Observed differences:
- OpenAI strict mode has extra constraints (`additionalProperties: false`, all fields required).
- Anthropic requires `input_schema`, strictness semantics less explicit.
- Gemini uses OpenAPI-like schema and mode-based adherence.

Lowest common denominator (recommended):
- `type: object`
- `properties` using primitives/arrays/objects
- `required`
- `enum`
- `additionalProperties: false`
- Avoid advanced combinators (`oneOf`/`anyOf`) in MVP.

Canonical Evidra Tool Schemas:
- `validate_plan` input:
  - `kind` (`terraform|k8s|generic`)
  - `payload` (object or file reference object)
  - `profile` (string, optional default)
  - `context` (object)
- `get_evidence` input:
  - `evidence_id` (string)

Adapter rules:
- OpenAI: set `strict: true` and satisfy strict-mode constraints.
- Anthropic: reuse same schema as `input_schema`.
- Gemini: translate to declaration schema shape (`OBJECT/STRING/...` enums as required by SDK/API).

## 7) Security Considerations (Local Mode)

1. Treat infra plans/manifests as sensitive; avoid logging full payloads.
2. Restrict evidence store permissions (owner-only where possible).
3. Use temp files with secure permissions and cleanup.
4. Avoid shell injection in wrapper command execution:
   - no shell interpolation
   - pass args as array.
5. Validate payload size before invocation to prevent memory pressure.

## 8) Compatibility Test Matrix

| Platform | Strategy | Tool | Expected success | Expected failure | Edge cases |
|---|---|---|---|---|---|
| OpenAI | Direct CLI | validate_plan | Tool call generated; wrapper returns deterministic decision JSON | Invalid payload -> structured error result | large payload file, invalid JSON, unknown profile |
| OpenAI | Direct CLI | get_evidence | Returns record by `evidence_id` | Unknown ID -> not_found style error | rotated/segmented evidence files |
| Anthropic | Direct CLI | validate_plan | `tool_use` then `tool_result` with decision JSON | schema mismatch -> client-side validation error | very long hints/reasons |
| Anthropic | Direct CLI | get_evidence | tool result includes evidence record | malformed id -> validation error | evidence store lock/busy |
| Gemini | Direct CLI | validate_plan | function call emitted (mode AUTO/ANY), response accepted | unsupported schema shape -> declaration error | mode differences (`AUTO` vs `ANY`) |
| Gemini | Direct CLI | get_evidence | function response carries evidence record | missing evidence id -> function-level error payload | high-frequency calls |
| Any | Wrapper service | validate_plan | stable HTTP/IPC contract | service unavailable -> retriable error | timeout tuning |
| Any | Embedded library | validate_plan | direct typed call success | dependency/version mismatch | API drift across releases |

## 9) Recommended MVP Path

Recommended first platform: **OpenAI API function calling**.
Recommended first strategy: **Direct CLI execution wrapper**.

Why:
- Most explicit strict schema controls.
- Clear invocation loop and tool-choice controls.
- Fastest path using existing `evidra validate --json`.

Two-week MVP plan:
1. Week 1:
   - Freeze `validate_plan` JSON schema + fixture outputs.
   - Build wrapper for direct CLI execution.
   - Add deterministic tests (same input => same output).
2. Week 2:
   - Implement `get_evidence` command or equivalent stable lookup surface.
   - Add provider adapters (OpenAI first, Anthropic second, Gemini third).
   - Publish examples and troubleshooting docs.

---

## Sources (Primary)

### OpenAI
- https://platform.openai.com/docs/guides/function-calling
- https://platform.openai.com/docs/guides/structured-outputs
- https://platform.openai.com/docs/guides/streaming-responses
- https://developers.openai.com/codex/

### Anthropic
- https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/overview
- https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/implement-tool-use
- https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/fine-grained-tool-streaming

### Google
- https://ai.google.dev/gemini-api/docs/function-calling
- https://ai.google.dev/gemini-api/docs/tokens
