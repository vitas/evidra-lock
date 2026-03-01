# MCP Inspector CLI Tests (Layer 2.5)

Deterministic integration tests for `evidra-mcp` using the [MCP Inspector CLI](https://github.com/modelcontextprotocol/inspector).

Sits between Go stdio integration tests (Layer 2, raw JSON-RPC) and agent E2E tests (Layer 3, Claude + LLM). No LLM, no Docker.

## Modes

The runner supports three modes via `EVIDRA_TEST_MODE`:

```
                local                  hosted                   rest
                ─────                  ──────                   ────
                Inspector CLI          Inspector CLI             curl
                    │                      │                      │
                stdio                  streamable-http        POST /v1/validate
                    │                      │                      │
                evidra-mcp             supergateway             evidra-api
                                           │
                                       evidra-mcp (stdio)

                MCP protocol           MCP protocol (same!)    HTTP/JSON (different)
                same tools             same tools              validate only
                same response          same response           jq normalize
```

`hosted` uses the same Inspector CLI and assertions as `local` — the only
difference is `CONFIG` contains `"type": "streamable-http"` instead of stdio.

`rest` normalizes the API response shape to match the MCP output format via jq.
MCP-only tests (`list_tools`, `get_event` chain) are skipped in rest mode.

## Prerequisites

| Mode | Required |
|---|---|
| local | Node.js >= 18 (`npx`), `jq`, Go toolchain (binary built from source) |
| hosted | Node.js >= 18 (`npx`), `jq`, `EVIDRA_MCP_URL` |
| rest | `curl`, `jq`, `EVIDRA_API_URL`, `EVIDRA_API_KEY` |

## Running

```bash
# Local (default) — builds from source, zero config
make test-mcp-inspector

# Hosted — requires running supergateway endpoint
EVIDRA_MCP_URL=https://evidra.samebits.com/mcp make test-mcp-inspector-hosted

# REST — requires running API server + API key
EVIDRA_API_URL=https://api.evidra.rest EVIDRA_API_KEY=... make test-mcp-inspector-rest

# or directly with env var
EVIDRA_TEST_MODE=hosted EVIDRA_MCP_URL=... bash tests/inspector/run_inspector_tests.sh
```

## Configuration

| Env var | Default | Description |
|---|---|---|
| `EVIDRA_TEST_MODE` | `local` | `local`, `hosted`, or `rest` |
| `EVIDRA_MCP_URL` | — | Streamable-http endpoint (hosted mode) |
| `EVIDRA_API_URL` | — | REST API base URL (rest mode) |
| `EVIDRA_API_KEY` | — | API key for REST auth (rest mode) |
| `EVIDRA_TEST_RETRIES` | `3` | Max retry attempts for rate-limited calls (hosted/rest) |
| `EVIDRA_TEST_RETRY_DELAY` | `2` | Base delay in seconds, multiplied by attempt (hosted/rest) |

Local mode never retries — no network, no rate limits.

## Architecture

### Special cases (`special/t_*.sh`)

Sourced by the main runner. Test things the corpus can't express:

| Script | What it tests | Modes |
|---|---|---|
| `t_list_tools.sh` | Tool registration, schema required fields | local, hosted |
| `t_get_event_chain.sh` | validate → get_event round-trip | local, hosted |
| `t_schema_error.sh` | Invalid input → error (MCP) or HTTP 400 (REST) | all |

### Corpus-driven loop

For each `tests/corpus/*.json` with both `input` and `expect` fields:

1. Transforms `.input` for the MCP path (flattens `params.action.*` to `params.*`)
2. Passes environment via inspector `-e EVIDRA_ENVIRONMENT=<env>` flag
3. Calls `validate` via transport abstraction (`call_validate`)
4. Asserts response fields against `.expect`

Agent-only cases (no `input`/`expect`) are skipped.

### Transport abstraction

`call_validate`, `call_get_event`, `call_list_tools` abstract over mode:

- **local** — `inspector_call_tool` → `extract_body` (no retry)
- **hosted** — `inspector_call_tool` → `extract_body` (with retry)
- **rest** — `curl` → jq normalization (with retry)

### Input transformation

Corpus files structure params as `params.action.{payload, target, risk_tags}` for direct OPA evaluation (used by Go corpus tests). The MCP server's `invocationToScenario` reads `params.{payload, target, risk_tags}` at the top level, so the runner flattens them. String targets are wrapped as `{"namespace": value}`.

### Assertion mapping

| Corpus `expect` field | Assertion target |
|---|---|
| (always) | `.event_id` is non-empty |
| `expect.allow` | `.policy.allow == value` |
| `expect.risk_level` | `.policy.risk_level == value` (skipped for allow+non-low) |
| `expect.rule_ids_contain[]` | each rule in `.rule_ids` |
| `expect.rule_ids_absent[]` | each rule NOT in `.rule_ids` |
| `expect.hints_min_count` | `.hints \| length >= N` |

### Known limitations

- **risk_level for warn rules**: The MCP scenario evaluation path returns `risk_level=low` for `allow=true` decisions, even when OPA computes a higher level (e.g. warn rules that set `medium`). The risk_level assertion is skipped for these cases. The Go corpus test validates the correct OPA-level risk_level.

## Adding tests

Add a new corpus file to `tests/corpus/` with `input` and `expect` fields — it will be picked up automatically in all modes. For special cases that can't be expressed as corpus files, add a `special/t_*.sh` script.
