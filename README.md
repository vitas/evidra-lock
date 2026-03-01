# Evidra

Kill-switch for AI agents managing infrastructure.

Your infrastructure. Your rules. Your evidence.

AI can suggest. Evidra decides.

- **Kill-switch** - destructive operations are blocked unless proven safe.
  Empty payload? Denied. Unknown tool? Denied. Ambiguous target scope? Denied.
- **Ops layer** - curated ops rules that catch the configs behind
  real outages: privileged containers, public S3, wildcard IAM, open
  security groups. Extensible - add your own rules.
- **Evidence** - every decision (allow and deny) is appended to a
  SHA-256 hash-chained log. Each record references the hash of the
  previous record. Tampering breaks verification.

---

## Why

AI agents now run `kubectl`, `terraform`, and `argocd` on production
infrastructure. They may produce incomplete, unsafe, or semantically
incorrect actions.
Humans may approve without understanding full impact. CI pipelines
may apply plans automatically.

Evidra sits between the agent and your infrastructure. Every
destructive command is validated against explicit policy before
execution. If it's dangerous, incomplete, or unknown, it's denied.

Evidra does not rely on natural language analysis. It evaluates
structured tool invocations against OPA policy. If the input cannot
be mapped to policy with sufficient context, the request is denied.

Input must be structured. Evidra does not parse raw shell commands.
Tool + operation + payload are explicit.

---

## Fail-closed by default

If Evidra cannot evaluate the operation safely, it denies.

- Unknown tool or operation -> deny
- Missing payload fields on destructive operation -> deny
- Truncated or incomplete input -> deny
- Ambiguous target scope (namespace not provided and cannot be inferred) -> deny

Scope can be provided explicitly (`target.namespace`) or via resolved
context (`context.namespace` from current kubectl/oc context).

Silence is not allow. Explicit allow is required for execution.
Read-only operations (`get`, `list`, `plan`, `describe`) are allowed by default.

---

## 30-Second Demo

1. Install Evidra. Connect to Claude Code. Try to break things.
2. Open Claude Code and type:

> "Delete all pods in the kube-system namespace."

3. Claude calls the `validate` MCP tool before acting. Evidra evaluates the action against policy.

4. Claude receives:

```json
{
  "allow": false,
  "risk_level": "high",
  "hits": ["k8s.protected_namespace"],
  "hints": [
    "Add risk_tag: breakglass",
    "Or apply changes outside kube-system"
  ]
}
```

5. Claude stops. Nothing is deleted. The denial and the hint are recorded in the evidence chain.

Try more prompts:

| Prompt | Expected result |
|---|---|
| "Delete all pods in kube-system" | DENY - `k8s.protected_namespace` |
| "Create a public S3 bucket" | DENY - `terraform.s3_public_access`, `aws_s3.no_encryption` |
| "Deploy nginx to the default namespace" | PASS - no rules matched |
| "Open SSH to 0.0.0.0/0" | DENY - `terraform.sg_open_world` |
| "Run a privileged container" | DENY - `k8s.privileged_container` |

Every decision - allow or deny - is written to `~/.evidra/evidence` as an append-only, hash-linked JSONL record.

---

## Install & Setup

Use the dedicated setup guide for install methods and MCP client configuration:

- [docs/EVIDRA_MCP_SETUP_GUIDE.md](docs/EVIDRA_MCP_SETUP_GUIDE.md)
- [GitHub version](https://github.com/vitas/evidra/blob/main/docs/EVIDRA_MCP_SETUP_GUIDE.md)

Binary downloads are available on [GitHub Releases](https://github.com/vitas/evidra/releases).

---

## Hosted Endpoints

| Endpoint | URL |
|---|---|
| MCP server | `https://evidra.samebits.com/mcp` |
| REST API | `https://api.evidra.rest/v1` |
| Landing / Console | `https://evidra.samebits.com` |

---

## UI Dev Mock Mode (No Backend)

If you want to review the UI locally without running `evidra-api`, start the UI with:

```bash
cd ui
VITE_MOCK_API=1 npm run dev
```

This enables a mocked API for:

- `POST /v1/keys`
- `POST /v1/validate`
- `GET /v1/evidence/pubkey`

Safety guardrails:

- Mock mode is enabled only when **both** are true:
  - `VITE_MOCK_API=1`
  - Vite mode is `development`
- Production builds (`npm --prefix ui run build`) do **not** enable mock mode.
- To disable locally, unset `VITE_MOCK_API` (or set it to `0`).

---

## How It Works

Evidra runs as a standard MCP server. AI agents discover it
automatically and call `validate` before destructive operations.

```
AI agent -> MCP: validate -> Evidra (OPA policy) -> allow/deny -> evidence chain
                                               |
                                   only if allowed -> kubectl / terraform / helm
```

1. Agent sends tool invocation via MCP before executing.
2. Evidra maps tool + operation to intent (destructive or read-only).
3. Request is evaluated against a versioned OPA policy bundle.
4. Decision returned: allow/deny + risk level + rule IDs + hints.
5. Decision recorded to append-only, hash-linked evidence log.

Deterministic and fail-closed. No LLM in the evaluation loop.
No runtime API calls. No network dependencies.
The policy bundle is versioned and ships embedded in the binary.

The agent sees rule IDs and actionable hints on every deny,
not just "no."

Also works in CI pipelines:

```bash
terraform show -json tfplan | evidra validate --tool terraform --op apply
```

Same policy engine. Same evidence chain. Works in AI workflows and
traditional CI pipelines. In CI mode, Evidra operates without MCP -
the same validation engine is called directly.

---

## Protection levels

Evidra ships with two levels. Default is maximum safety.

**ops** (default) - full protection. Kill-switch guardrails plus
curated ops rules that catch privileged containers, public S3 buckets,
wildcard IAM, open security groups, and other catastrophic
misconfigurations. Extensible - add your own rules.

**baseline** - kill-switch only. Blocks destructive operations with
missing context and unknown tools. No opinion on what's "bad config."

Switch with one env var:

```bash
EVIDRA_PROFILE=baseline  # kill-switch only
EVIDRA_PROFILE=ops       # default - full protection
```

---

## Policy catalog

Not a compliance scanner. Every rule prevents a specific high-impact
failure that has caused real outages.

Evidra ships with `ops-v0.1`: curated ops rules (deny + warn) covering Kubernetes, Terraform, ArgoCD, S3, and IAM. The ops layer is extensible - add your own rules.

Design principles:

- **Catastrophic only** - no hygiene rules, no best-practice noise.
- **Deterministic** - evaluated from static configuration without runtime API calls.
- **Low false-positive rate** - every rule maps to a known attack chain or incident class.

See [docs/POLICY_CATALOG.md](docs/POLICY_CATALOG.md) for the full rule catalog.

---

## Not a replacement for OPA

Admission controllers run at deploy time, inside the cluster.
Evidra runs before execution - across `kubectl`, `terraform`, `helm`,
`argocd`. Especially in AI-driven workflows. Keep both.

---

## Threat model

Evidra assumes:

- AI agents may generate incomplete or unsafe infrastructure actions.
- Payload may be incorrect, partial, or adversarial.
- Humans may approve without understanding full impact.
- CI pipelines may apply plans automatically.

Evidra does not:

- Execute commands.
- Modify infrastructure.
- Make network calls during evaluation.
- Replace admission controllers or CI gates.

It validates structured input against explicit policy, and records
the decision. Nothing more.

Evidra does not sit in the execution path. It must be called
before execution by the agent, CLI, or CI pipeline.

---

## Evidence

Every decision - allow and deny - is recorded to a local,
append-only evidence log.

- SHA-256 hash chain: each record includes the hash of the previous record.
- Immutable: append-only file, no updates, no deletes.
- Tamper detection: `evidra evidence verify` checks the full chain.
- Offline: stored locally, no external service required.
- Contains: actor, operation, policy decision, rule IDs, timestamps, payload digest (not raw payload by default). Raw payload storage is optional and configurable.

```bash
# Verify the evidence chain is intact
evidra evidence verify

# Export for external audit
evidra evidence export --format json
```

---

## CLI (Policy Debugging & Evidence Tools)

The `evidra` CLI shares the same evaluation engine as the MCP server. It is primarily intended for policy development and debugging.

```bash
# Validate a scenario file against policy
evidra validate examples/demo/kubernetes_kube_system_delete.json

# Verify evidence chain integrity
evidra evidence verify

# Export evidence for external audit
evidra evidence export --format json
```

Exit codes: `0` = PASS, `2` = FAIL, `1` = error.

---

## License

Apache 2.0. No SaaS required. No telemetry. Runs locally or on-prem.

Your infrastructure, your rules, your evidence.

---
Open source by [SameBits](https://samebits.com).
