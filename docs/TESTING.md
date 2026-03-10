# Testing

## Quick Reference

| Command | What | Speed | Cost |
|---|---|---|---|
| `make test` | Go unit tests + OPA policy evaluation | ~5s | free |
| `make test-corpus` | Corpus-driven Go tests (all policy rules) | ~2s | free |
| `make validate-corpus` | Corpus integrity (JSON, unique IDs, catalog sync) | ~1s | free |
| `make test-mcp-inspector` | MCP protocol via Inspector CLI (local stdio) | ~60s | free |
| `make test-mcp-inspector-hosted` | MCP protocol via Inspector CLI (hosted streamable-http) | ~90s | free |
| `make test-mcp-inspector-rest` | REST API `/v1/validate` via curl | ~30s | free |
| `make test-skill-e2e` | Agent e2e — Claude + skill (p0 cases) | ~60s | ~$0.15 |
| `make test-skill-e2e-full` | Agent e2e — all scenarios | ~5min | ~$1.50 |

Run everything locally before a PR:

    make test && make validate-corpus && make test-mcp-inspector

---

## Architecture

```
Layer 1: OPA policy unit tests
    policy/bundles/ops-v0.1/tests/*.rego
    Tests: policy logic in isolation
    Run: opa test policy/bundles/ops-v0.1/ -v
    Data: inline in .rego files

Layer 2: Go integration tests
    cmd/evidra-mcp/test/stdio_integration_test.go
    Tests: MCP wire protocol, JSON-RPC, tool schemas
    Run: go test ./cmd/evidra-mcp/test/...
    Data: cmd/evidra-mcp/test/testdata/*.jsonl

Layer 2.5: MCP Inspector CLI tests
    tests/inspector/run_inspector_tests.sh
    Tests: MCP tool behavior (validate, get_event) via official Inspector
    Run: make test-mcp-inspector
    Data: tests/corpus/*.json (shared)

Layer 3: Agent e2e tests
    tests/e2e/run_e2e.sh
    Tests: AI agent behavior with evidra skill
    Run: make test-skill-e2e
    Data: tests/corpus/*.json .agent section
    Requires: ANTHROPIC_API_KEY, claude CLI
```

Each layer builds on the previous. Run them in order — if layer 1 fails,
don't bother with layer 3.

---

## Test Corpus

All policy test cases live in `tests/corpus/`. One JSON file per case.
Inspector tests and agent e2e read from the corpus — no hardcoded payloads.

```
tests/corpus/
├── k8s_privileged_container_deny.json
├── k8s_safe_manifest_allow.json
├── ops_mass_delete_deny.json
├── ...
├── manifest.json              # coverage tracking
├── corpus_test.go             # Go test consumer
└── scripts/
    ├── validate_corpus.sh     # integrity checker
    └── coverage_report.sh     # rule coverage reporter
```

Each file has up to four sections:

| Section | Purpose | Used by |
|---|---|---|
| `_meta` | Case ID, rule IDs, priority | all layers |
| `input` | ToolInvocation payload | Go tests, Inspector |
| `expect` | Assertions (allow, risk_level, rule_ids, hints) | Go tests, Inspector |
| `agent` | Prompt + LLM-specific expectations | agent e2e only |

Files with only `_meta` + `agent` (no `input`/`expect`) are agent-only cases
that test behavior outside the ToolInvocation evaluation path.

Adding a corpus file automatically adds tests to the Inspector and e2e layers.

Validate corpus integrity:

    make validate-corpus

Coverage report (cross-reference with policy catalog):

    make corpus-coverage

See [tests/corpus/README.md](/tests/corpus/README.md) for the JSON format.

---

## Layer 1: OPA Policy Tests

Test policy rules in isolation using OPA's built-in test runner.

    opa test policy/bundles/ops-v0.1/ -v

Tests live in `policy/bundles/ops-v0.1/tests/`. One test file per rule.
See [docs/CONTRIBUTING.md](CONTRIBUTING.md) § "Adding a Policy Rule" for conventions.

---

## Layer 2: Go Integration Tests

Test Go packages and MCP wire protocol (JSON-RPC over stdio).

    go test ./...                         # all packages
    go test -race ./...                   # with race detector (CI default)
    go test ./cmd/evidra-mcp/test/...     # MCP stdio integration only
    go test -run TestFoo ./pkg/validate   # single test

Test data: `cmd/evidra-mcp/test/testdata/*.jsonl` (MCP wire fixtures).

The corpus-driven Go tests run separately:

    make test-corpus

These evaluate every `tests/corpus/*.json` with `input`/`expect` fields
through the OPA engine directly (`runtime.EvaluateInvocation`).

---

## Layer 2.5: MCP Inspector Tests

Deterministic MCP tool-call tests using the
[MCP Inspector CLI](https://github.com/modelcontextprotocol/inspector).
No LLM, no Docker.

### Modes

The runner supports three modes via `EVIDRA_TEST_MODE`:

| Mode | Transport | Target | Command |
|---|---|---|---|
| `local` (default) | Inspector CLI → stdio | `evidra-lock-mcp` binary | `make test-mcp-inspector` |
| `hosted` | Inspector CLI → streamable-http | supergateway → `evidra-lock-mcp` | `make test-mcp-inspector-hosted` |
| `rest` | curl → HTTP | `evidra-lock-api` REST `/v1/validate` | `make test-mcp-inspector-rest` |

`local` and `hosted` use the same MCP protocol — the only difference is transport
(stdio vs streamable-http). `rest` uses a different response shape and normalizes
via jq. MCP-only tests (`list_tools`, `get_event` chain) are skipped in rest mode.

```bash
# Local (default) — builds from source, no env vars needed
make test-mcp-inspector

# Hosted — requires running supergateway endpoint
EVIDRA_MCP_URL=https://evidra.samebits.com/mcp make test-mcp-inspector-hosted

# REST — requires running API server + API key
EVIDRA_API_URL=https://api.evidra.rest EVIDRA_API_KEY=... make test-mcp-inspector-rest
```

### Prerequisites

| Mode | Required |
|---|---|
| local | Node.js >= 18 (`npx`), `jq`, Go toolchain |
| hosted | Node.js >= 18 (`npx`), `jq`, `EVIDRA_MCP_URL` |
| rest | `curl`, `jq`, `EVIDRA_API_URL`, `EVIDRA_API_KEY` |

### Retry (hosted/rest)

Network modes retry on 429/rate-limit/transient errors with linear backoff.
Configurable via `EVIDRA_TEST_RETRIES` (default 3) and `EVIDRA_TEST_RETRY_DELAY`
(default 2s, multiplied by attempt number). Local mode never retries.

### Test structure

- **Corpus loop** — iterates `tests/corpus/*.json`, transforms input for the
  MCP scenario path, calls `validate` via Inspector, asserts `.expect` fields.
- **Special cases** — tool registration schemas, `get_event` round-trip,
  invalid input handling.

The runner transforms corpus `params.action.*` to flat `params.*` because the
MCP server's `invocationToScenario` reads params at the top level. Environment
is passed via the Inspector's `-e` flag for environment-dependent policy params.

See [tests/inspector/README.md](/tests/inspector/README.md) for details.

---

## Layer 3: Agent E2E Tests

Test AI agent behavior with the evidra skill using Claude Code headless.

    # p0 cases only (fast, cheap)
    make test-skill-e2e

    # all scenarios
    make test-skill-e2e-full

    # all scenarios with retry (majority vote, for CI weekly)
    make test-skill-e2e-weekly

Requires: `ANTHROPIC_API_KEY`, `claude` CLI.

Reads `tests/corpus/*.json` files with an `.agent` section. Each case provides
a natural language prompt and expected agent behaviors:

- `expect_validate_called` — agent invoked the validate tool
- `expect_allow` — policy decision matches
- `expect_no_mutation` — agent did not execute destructive commands
- `expect_stop_signal` — agent showed denial to the user
- `expect_evidence_deny` — evidence store contains deny record

System prompt source of truth for e2e runner:
`skills/evidra-infra-safety/prompts/system_prompt.txt`.

The runner supports model selection (`MODEL=haiku|sonnet|opus`), online/offline
modes, and generates an HTML report.

Model behavior note:
- Stronger models are usually more predictable in E2E tool-use behavior (typically `opus` > `sonnet` > `haiku`), but nondeterminism still exists.
- E2E no longer enforces payload byte-size thresholds; it focuses on behavior/tool-use assertions.
- Per-scenario output now records tool usage (`tool_usage` + `output_file`) in `results.ndjson`, and the HTML report shows tool usage counts per scenario.
- The HTML report includes a clickable `Open Agent NDJSON Trace` link per scenario so you can inspect exactly what the agent sent.
- Example failure analysis from a full haiku run: [docs/E2E_HAIKU_FULL_FAILURE_ANALYSIS.md](docs/E2E_HAIKU_FULL_FAILURE_ANALYSIS.md).

Non-deterministic (LLM). Run only after layers 1–2.5 pass.

---

## CI Order

```
1. make test                     # Go + OPA (fast, free)
2. make validate-corpus          # corpus integrity (fast, free)
3. make test-mcp-inspector       # MCP protocol (medium, free)
4. make test-skill-e2e           # agent e2e (slow, paid — only if 1-3 pass)
```

---

## Adding Tests

**New policy rule:**

1. Add `.rego` rule in `policy/bundles/ops-v0.1/evidra/policy/rules/`
2. Add OPA test in `policy/bundles/ops-v0.1/tests/`
3. Add corpus case in `tests/corpus/`
4. Inspector and e2e pick it up automatically

**New corpus case:**

Create a JSON file in `tests/corpus/` with `_meta`, `input`, and `expect`.
Add an `agent` section if you want e2e coverage.
Run `make validate-corpus` to verify.

**New special case (Inspector):**

Add a `t_*.sh` script in `tests/inspector/special/`.
It will be sourced by the runner automatically.
