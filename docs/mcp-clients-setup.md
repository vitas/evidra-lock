# MCP Client Setup for Evidra MCP server

## 1) Overview

**Evidra MCP server** receives requests from an AI client, evaluates them against deterministic policy (OPA), and writes an audit trail to the evidence store.
The client (Codex / Gemini / Claude Desktop) acts as an MCP consumer: it calls `validate`, receives the decision, and then continues or stops.

Flow:

```text
Agent/Client -> Evidra MCP -> Decision + EvidenceID -> Agent continues/stops
```

## 2) Prerequisites

### Build binaries

From source (`evidra-mcp` repo):

```bash
go build -o ./bin/evidra ./cmd/evidra
go build -o ./bin/evidra-mcp ./cmd/evidra-mcp
```

`make build` currently builds only `evidra`:

```bash
make build
```

### Supported OS

- `evidra` release targets include `linux`, `darwin`, `windows` (arch depends on release/build target).
- `evidra-mcp` support depends on your Go build targets.

### Verify installation

```bash
./bin/evidra version
./bin/evidra-mcp --help
```

### Default evidence store path

- Canonical default: `~/.evidra/evidence`
- Override:
  - server flag: `--evidence-store` (alias: `--evidence-dir`)
  - env: `EVIDRA_EVIDENCE_DIR`

## 3) Running the server (common)

### Local foreground run

```bash
./bin/evidra-mcp \
  --bundle ./policy/bundles/ops-v0.1
```

### Run with explicit evidence store

```bash
./bin/evidra-mcp \
  --bundle ./policy/bundles/ops-v0.1 \
  --evidence-store /var/lib/evidra/evidence
```

Or with env:

```bash
export EVIDRA_EVIDENCE_DIR=/var/lib/evidra/evidence
./bin/evidra-mcp --bundle ./policy/bundles/ops-v0.1
```

### Run with environment label

```bash
./bin/evidra-mcp \
  --bundle ./policy/bundles/ops-v0.1 \
  --environment production
```

### Observe mode

```bash
./bin/evidra-mcp \
  --bundle ./policy/bundles/ops-v0.1 \
  --observe
```

### Logging verbosity

There is no dedicated `--log-level` flag at the moment. Server logs are written to process stderr/stdout.

### Health check / test call

Use CLI validation as a quick end-to-end check:

```bash
./bin/evidra validate ./examples/terraform_plan_pass.json
./bin/evidra validate --json ./examples/terraform_public_exposure_fail.json
```

Expected output shape:
- text: `Decision`, `Risk level`, `Evidence`, `Reason`; on FAIL also `Rule IDs` and `How to fix`
- json: `status`, `risk_level`, `reason`, `reasons`, `rule_ids`, `hints`, `evidence_id`

Samples exist under `examples/` (including `examples/invocations/`).

## 4) Client setup: OpenAI Codex

### Steps

1. Run `evidra-mcp` locally (section above).
2. Add MCP server definition to your Codex MCP config.
3. Verify that tool `validate` appears in the available tools list.

> Codex MCP config location depends on your environment. Use the JSON snippet below as the server definition.

### Minimal config snippet

```json
{
  "name": "evidra",
  "command": "/absolute/path/to/evidra-mcp",
  "args": [
    "--bundle", "/absolute/path/to/policy/bundles/ops-v0.1",
    "--evidence-store", "/absolute/path/to/evidence-store"
  ],
  "env": {
    "EVIDRA_MODE": "enforce"
  }
}
```

### Minimal tool call

```json
{
  "tool": "validate",
  "input": {
    "actor": { "type": "agent", "id": "codex", "origin": "mcp" },
    "tool": "kubectl",
    "operation": "delete",
    "params": {
      "payload": { "namespace": "prod", "resource_count": 7 },
      "risk_tags": []
    },
    "context": { "source": "mcp", "scenario_id": "codex-demo-1" }
  }
}
```

Expected response shape:

```json
{
  "ok": false,
  "event_id": "evt-...",
  "policy": {
    "allow": false,
    "risk_level": "high",
    "reason": "Changes in protected namespace require change-approved tag"
  },
  "rule_ids": ["ops.unapproved_change"],
  "reasons": ["Changes in protected namespace require change-approved tag"],
  "hints": ["Add risk_tag: change-approved", "..."]
}
```

## 5) Client setup: Gemini

### MCP compatibility note

If your Gemini runtime does not provide direct MCP transport, use an **MCP bridge/client wrapper** that can host stdio MCP servers and forward tool calls.

### Minimal bridge-style config

```json
{
  "mcpServer": {
    "command": "/absolute/path/to/evidra-mcp",
    "args": [
      "--bundle", "/absolute/path/to/policy/bundles/ops-v0.1",
      "--evidence-store", "/absolute/path/to/evidence-store"
    ],
    "env": {
      "EVIDRA_MODE": "enforce"
    }
  }
}
```

### Tool call and response

Use the same `validate` input shape as in Codex (`actor/tool/operation/params/context`).

Expected response fields:
- `ok`, `policy.allow`, `policy.risk_level`
- `rule_ids`, `reasons`, `hints`
- `event_id` for audit lookup

## 6) Client setup: Claude Desktop

### Steps

1. Build/install `evidra-mcp`.
2. Add server entry in Claude Desktop MCP config.
3. Restart Claude Desktop.
4. Call tool `validate`.

### Full JSON snippet (`mcpServers`)

```json
{
  "mcpServers": {
    "evidra": {
      "command": "/absolute/path/to/evidra-mcp",
      "args": [
        "--bundle", "/absolute/path/to/policy/bundles/ops-v0.1",
        "--evidence-store", "/absolute/path/to/evidence-store"
      ],
      "env": {
        "EVIDRA_MODE": "enforce"
      }
    }
  }
}
```

### OS notes

- **Windows**: use Windows paths (`C:\\...`) and proper JSON escaping.
- **macOS/Linux**: use absolute POSIX paths (`/Users/...`, `/home/...`).
- In all cases, prefer absolute path to `evidra-mcp`.

## 7) Security & operational notes

- Run Evidra MCP server with least privilege.
- Evidence store contains audit records; enforce filesystem access control and backups.
- Pin policy files and use review workflow (PR + tests) before policy changes.
- Start rollout with `--observe`, then switch to enforce.
- Evidence is hash-chained; verify integrity regularly:

```bash
./bin/evidra evidence verify --evidence ~/.evidra/evidence
```

## 8) Troubleshooting

### Server not found / command not in PATH

- Use absolute binary path in MCP config.
- Check:

```bash
which evidra-mcp
./bin/evidra-mcp --help
```

### Permission denied on evidence store path

- Check directory permissions.
- Override to a writable path:

```bash
export EVIDRA_EVIDENCE_DIR=/tmp/evidra-evidence
```

### Client cannot connect

- Verify MCP config `command` and `args`.
- Check `evidra-mcp` stderr logs for startup errors.
- Ensure `--bundle` points to an existing directory with a valid `.manifest`.

### Unexpected policy deny

- Reproduce locally:

```bash
./bin/evidra validate --explain ./examples/terraform_public_exposure_fail.json
```

- Use `event_id` from output and inspect evidence:

```bash
rg 'evt-' ~/.evidra/evidence/segments/*.jsonl
```

- From MCP side, query related evidence via `get_event`.

## 9) Appendices

### A) Common config template

```json
{
  "command": "/absolute/path/to/evidra-mcp",
  "args": [
    "--bundle", "/absolute/path/to/policy/bundles/ops-v0.1",
    "--evidence-store", "/absolute/path/to/evidence-store",
    "--observe"
  ],
  "env": {
    "EVIDRA_MODE": "observe"
  }
}
```

### B) Minimal end-to-end demo

1. Start server:

```bash
./bin/evidra-mcp \
  --bundle ./policy/bundles/ops-v0.1 \
  --evidence-store ~/.evidra/evidence
```

2. From client, call `validate` with a deny scenario (for example: `kubectl.delete` in `prod` without `change-approved`).
3. Confirm response contains `ok=false`, `rule_ids`, `reasons`, `hints`, `event_id`.
4. Call `validate` with an allow scenario (for example: safe `kubectl.apply` in `default`) and confirm `ok=true`.
5. Verify evidence chain:

```bash
./bin/evidra evidence verify --evidence ~/.evidra/evidence
```

6. Locate event by id:

```bash
rg '<event_id>' ~/.evidra/evidence/segments/*.jsonl
```
