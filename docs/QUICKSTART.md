# Quickstart (5-10 Minutes)

This guide runs Evidra locally in ops profile, simulates policy, verifies evidence, and exports an audit pack.

## 1) Build

```bash
go build -o ./bin/evidra-mcp ./cmd/evidra-mcp
go build -o ./bin/evidra-policy-sim ./cmd/evidra-policy-sim
go build -o ./bin/evidra-evidence ./cmd/evidra-evidence
```

## 2) Start MCP Server (Ops Profile)

```bash
EVIDRA_PROFILE=ops \
EVIDRA_EVIDENCE_PATH=./data/evidence \
./bin/evidra-mcp
```

Defaults:
- profile defaults to `ops` if unset.
- evidence defaults to `./data/evidence`.
- ops policy defaults to `./policy/kits/ops-v0.1/policy.rego`.
- ops data defaults to `./policy/kits/ops-v0.1/data.example.json`.
- ops packs default to `./packs/_core/ops`.

## 3) Offline Policy Simulation

```bash
./bin/evidra-policy-sim \
  --policy ./policy/kits/ops-v0.1/policy.rego \
  --data ./policy/kits/ops-v0.1/data.example.json \
  --input ./examples/invocations/allowed_kubectl_get_dev.json
```

## 4) Invoke Through MCP

Use any MCP client to call tool `execute` with canonical `ToolInvocation` JSON.
A successful call returns `event_id`, plus policy and execution summaries (`risk_level`, `reason`, status, exit_code).

Example payload:

```json
{
  "actor": {"type": "human", "id": "ops-user", "origin": "mcp"},
  "tool": "argocd",
  "operation": "app-get",
  "params": {"app": "payments-api"},
  "context": {"environment": "prod"}
}
```

Example success response shape:

```json
{
  "ok": true,
  "event_id": "evt-123",
  "policy": {"allow": true, "risk_level": "low", "reason": "allowed_by_rule", "policy_ref": "b4b6..."},
  "execution": {"status": "success", "exit_code": 0, "stdout": "...", "stderr": ""}
}
```

## 5) Fetch a Record by event_id

Call MCP tool `get_event`:

```json
{
  "event_id": "evt-..."
}
```

Expected wrapper response:

```json
{
  "ok": true,
  "record": {
    "event_id": "evt-...",
    "tool": "argocd",
    "operation": "app-get",
    "hash": "..."
  }
}
```

## 6) Evidence Integrity and Export

```bash
./bin/evidra-evidence verify --evidence ./data/evidence
./bin/evidra-evidence violations --evidence ./data/evidence --min-risk high
./bin/evidra-evidence export --evidence ./data/evidence --out ./audit-pack.tar.gz --policy ./policy/policy.rego --data ./policy/data.json
```

## 7) Expected Files

Under `./data/evidence`:
- `manifest.json`
- `segments/evidence-000001.jsonl` (and more as it rotates)
- `forwarder_state.json` only after cursor ack operations

## 8) Testing Guarded Mode Locally

- Start Evidra MCP (ops profile).
- Configure your MCP client/agent to use only Evidra MCP tools.
- Disable direct shell in the agent if your client supports it.
- Execute an operation through MCP (for example `kubectl` or `helm`).
- Verify integrity:

```bash
./bin/evidra-evidence verify --evidence ./data/evidence
```
