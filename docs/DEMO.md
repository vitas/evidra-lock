# Demo (Ops Guardrails)

## 1) Setup

Build binaries:

```bash
go build -o ./bin/evidra-mcp ./cmd/evidra-mcp
go build -o ./bin/evidra-policy-sim ./cmd/evidra-policy-sim
go build -o ./bin/evidra-evidence ./cmd/evidra-evidence
```

Run MCP server (ops profile):

```bash
EVIDRA_PROFILE=ops \
EVIDRA_PACKS_DIR=./packs/_core/ops \
EVIDRA_POLICY_PATH=./policy/kits/ops-v0.1/policy.rego \
EVIDRA_POLICY_DATA_PATH=./policy/kits/ops-v0.1/data.json \
EVIDRA_EVIDENCE_PATH=./data/evidence \
./bin/evidra-mcp
```

## 2) Scenario: Prod Write is Critical

Dev Helm upgrade invocation:

```json
{
  "actor": {"type": "human", "id": "ops-dev", "origin": "mcp"},
  "tool": "helm",
  "operation": "upgrade",
  "params": {"release": "payments", "chart": "./chart", "namespace": "payments-dev"},
  "context": {"environment": "dev", "cluster": "local"}
}
```

Prod Terraform apply invocation:

```json
{
  "actor": {"type": "human", "id": "ops-prod", "origin": "mcp"},
  "tool": "terraform",
  "operation": "apply",
  "params": {"dir": "./infra"},
  "context": {"environment": "prod", "cluster": "remote"}
}
```

Run policy simulation:

```bash
./bin/evidra-policy-sim --policy ./policy/kits/ops-v0.1/policy.rego --data ./policy/kits/ops-v0.1/data.json --input ./examples/invocations/allowed_terraform_apply_dev.json
./bin/evidra-policy-sim --policy ./policy/kits/ops-v0.1/policy.rego --data ./policy/kits/ops-v0.1/data.json --input ./examples/invocations/allowed_helm_upgrade_prod.json
```

Expected output fields:
- `allow`
- `risk_level`
- `reason`

When invoked through MCP `execute`, response also includes `event_id`.

## 3) Evidence Inspection

```bash
./bin/evidra-evidence violations --evidence ./data/evidence --min-risk high
./bin/evidra-evidence export --evidence ./data/evidence --out ./audit-pack.tar.gz --policy ./policy/kits/ops-v0.1/policy.rego --data ./policy/kits/ops-v0.1/data.json
```

The audit pack is a portable artifact for review and incident handoff.

## 4) Event Retrieval

Use returned `event_id` from `execute` and call MCP tool `get_event`.

UI/client can display:
- `policy_decision.risk_level`
- `policy_decision.reason`
- `policy_ref`
- execution status and exit code
