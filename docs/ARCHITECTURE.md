# Architecture

## Overview

Evidra has two entry points sharing a single evaluation core:

| Binary | Role | Interface |
|---|---|---|
| `evidra-mcp` | Primary — MCP server for AI agents | stdio (JSON-RPC) |
| `evidra` | Secondary — offline CLI for policy debugging and evidence tools | command-line |

Both call `pkg/validate.EvaluateScenario`, which loads a scenario, evaluates it against the OPA policy bundle, and writes an evidence record. Same policy, same decisions, same evidence format.

```
┌─────────────────┐     ┌─────────────────┐
│  evidra-mcp     │     │  evidra CLI     │
│  (MCP server)   │     │  (offline)      │
└────────┬────────┘     └────────┬────────┘
         │                       │
         └───────────┬───────────┘
                     │
              ┌──────┴──────┐
              │ pkg/validate │
              └──────┬──────┘
                     │
         ┌───────────┼───────────┐
         │           │           │
    ┌────┴────┐ ┌────┴────┐ ┌───┴────┐
    │scenario │ │ runtime │ │evidence│
    │ loader  │ │  (OPA)  │ │ store  │
    └─────────┘ └─────────┘ └────────┘
```

---

## MCP-First Design

Evidra is designed as a pre-execution validation layer for AI agents. The MCP server is the primary integration surface.

**Tools exposed:**

- `validate` — evaluates a tool invocation against policy, returns a structured decision, records evidence.
- `get_event` — fetches a single evidence record by `event_id`.

**Resources:**

- `evidra://event/{event_id}` — read a specific evidence record.
- `evidra://evidence/manifest` — read the evidence store manifest.
- `evidra://evidence/segments` — read sealed/current segment summary.

**Transport:** stdio. The server reads JSON-RPC from stdin and writes to stdout. No HTTP, no sockets.

**Modes:**

- `enforce` (default) — deny decisions block the action.
- `observe` (`--observe`) — policy is evaluated and recorded but never blocks.

The agent calls `validate` before executing an infrastructure operation. Evidra evaluates the invocation, returns a decision with rule IDs and hints, and records the outcome. Evidra does not execute the operation itself — it only validates.

---

## Embedded Policy Bundle

The MCP server ships with the `ops-v0.1` OPA bundle compiled into the binary via `go:embed`. When no `--bundle` flag or `EVIDRA_BUNDLE_PATH` environment variable is set, the server extracts the embedded bundle to a temp directory at startup.

Zero configuration is needed to start using Evidra: install the binary, point your MCP client at it, and the 23-rule baseline is active.

**Bundle structure:**

```
policy/bundles/ops-v0.1/
├── .manifest                    — revision, roots, profile metadata
├── evidra/policy/
│   ├── policy.rego              — decision entrypoint
│   ├── decision.rego            — deny/warn aggregator
│   ├── defaults.rego            — shared helpers (resolve_param, has_tag)
│   └── rules/                   — one .rego file per rule
├── evidra/data/params/          — tunable parameters (by_env model)
└── evidra/data/rule_hints/      — remediation hints per rule
```

Custom bundles can be supplied with `--bundle <path>` for development or alternative rule sets.

---

## Offline Design

All evaluation is deterministic and local:

- Input is static configuration data: Terraform plan JSON, Kubernetes manifests, ArgoCD sync policies.
- No network calls during evaluation. No external APIs. No cloud provider connections.
- Given the same input and policy bundle, the same decision is produced every time.
- The server works in air-gapped environments.

The OPA engine is embedded in the binary. Parameters are resolved from the bundle's `data.json` using a `by_env` fallback chain: environment-specific value → default value.

---

## Evidence Model

Every `validate` call produces an evidence record, regardless of outcome (allow or deny). Records are written to an append-only JSONL store at `~/.evidra/evidence` (configurable via `--evidence-dir` or `EVIDRA_EVIDENCE_DIR`).

Each record includes:

| Field | Content |
|---|---|
| Actor | Who initiated the invocation (human, agent, system) and origin (cli, mcp) |
| Action | Tool, operation, target, parameters |
| Decision | allow/deny, risk level, rule IDs, reasons, hints |
| Chain | `previous_hash` linking to the prior record, self-verifying `hash` |
| Metadata | Timestamps, event ID, policy reference |

The hash chain makes tampering detectable. If evidence cannot be written, the validation pipeline returns an error — the caller cannot bypass logging.

The store is segmented. Evidence can be verified and exported with the CLI:

```bash
evidra evidence verify       # validate hash chain integrity
evidra evidence export       # export for external audit systems
```

---

## Evaluation Pipeline

```
input → pkg/scenario (load/normalize) → pkg/runtime (OPA eval) → pkg/policy (Decision) → pkg/evidence (record)
```

1. **Scenario loading** (`pkg/scenario`): Normalizes input from Terraform plan JSON, Kubernetes manifests, or explicit action lists into a canonical action schema.
2. **Policy evaluation** (`pkg/runtime` + `pkg/policy`): Runs the action through the OPA engine against the active bundle. Returns a `Decision` with `allow`, `risk_level`, `reasons`, `hits`, and `hints`.
3. **Evidence recording** (`pkg/evidence`): Appends the decision as a hash-linked JSONL record.

---

## Key Packages

| Package | Role |
|---|---|
| `pkg/validate` | Central evaluation: loads scenario, runs policy, records evidence |
| `pkg/mcpserver` | MCP adapter: bridges tool invocations to `pkg/validate` |
| `pkg/scenario` | Scenario schema and file loader |
| `pkg/runtime` | OPA evaluator with `PolicySource` interface |
| `pkg/policy` | OPA engine wrapper; evaluates `data.evidra.policy.decision` |
| `pkg/evidence` | Append-only JSONL store with hash-linked chain |
| `pkg/config` | Resolves flags and `EVIDRA_*` env vars |
| `pkg/invocation` | Canonical `ToolInvocation` schema |
| `pkg/bundlesource` | Loads OPA bundle directories |
| `pkg/policysource` | Loads individual .rego + data.json files |

---

## Configuration

| Flag | Env Var | Default | Purpose |
|---|---|---|---|
| `--bundle` | `EVIDRA_BUNDLE_PATH` | embedded `ops-v0.1` | Policy bundle directory |
| `--evidence-dir` | `EVIDRA_EVIDENCE_DIR` | `~/.evidra/evidence` | Evidence store location |
| `--environment` | `EVIDRA_ENVIRONMENT` | — | Environment label for param resolution |
| `--observe` | `EVIDRA_MODE=observe` | `enforce` | Observe-only mode |
