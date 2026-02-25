# Evidra

Evidra is a guardrail layer between AI agents and your infrastructure.

- 23 OPA policy rules covering Kubernetes, Terraform, ArgoCD, S3, and IAM
- Structured deny responses with rule IDs and actionable hints
- Append-only, hash-linked evidence chain for every decision
- MCP-native: runs as a stdio server inside Claude Code, Cursor, or any MCP client

---

## Why It Exists

AI agents can now run `kubectl`, `terraform apply`, and `argocd sync` on production infrastructure. Without a validation layer in the loop, a single misinterpreted prompt can delete a namespace, open a security group to the world, or disable S3 encryption.

Evidra intercepts every tool invocation before it executes. If the action violates policy, the agent receives a structured denial with a rule ID and a hint — not a silent failure. The decision is recorded to a local evidence chain regardless of outcome.

---

## 30-Second Demo

1. Install Evidra and connect it to Claude Code (see below).
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
| "Delete all pods in kube-system" | DENY — `k8s.protected_namespace` |
| "Create a public S3 bucket" | DENY — `terraform.s3_public_access`, `aws_s3.no_encryption` |
| "Deploy nginx to the default namespace" | PASS — no rules matched |
| "Open SSH to 0.0.0.0/0" | DENY — `terraform.sg_open_world` |
| "Run a privileged container" | DENY — `k8s.privileged_container` |

Every decision — allow or deny — is written to `~/.evidra/evidence` as an append-only, hash-linked JSONL record.

---

## Install

**Homebrew** (macOS / Linux)

```bash
brew install vitas/tap/evidra-mcp
```

**Docker**

```bash
docker run --rm -i ghcr.io/vitas/evidra-mcp:latest
```

**Go install**

```bash
go install samebits.com/evidra/cmd/evidra-mcp@latest
```

Binary downloads available on [GitHub Releases](https://github.com/vitas/evidra/releases).

---

## Connect to Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp"
    }
  }
}
```

No extra flags required — the binary ships with the `ops-v0.1` policy bundle built in.

For other MCP clients, use the same `command` / `args` shape in their respective config.

---

## How It Works

```
AI agent → MCP validate tool → Policy engine (OPA) → Decision → Evidence chain
```

1. The MCP server receives a tool invocation from the AI agent.
2. The invocation is evaluated against the active OPA policy bundle.
3. A structured decision is returned: `allow`, `risk_level`, `reasons`, `hints`.
4. The decision is recorded as an append-only, hash-linked evidence record.

The agent receives rule IDs and hints so it can stop, retry with corrected parameters, or escalate to a human.

**Modes:**

- `enforce` (default) — deny blocks the action.
- `observe` (`--observe`) — policy is evaluated and recorded but never blocks.

---

## Policy Baseline

Evidra ships with `ops-v0.1`: 23 rules (18 deny, 5 warn) covering Kubernetes, Terraform, ArgoCD, S3, and IAM.

These are **catastrophic guardrails**, not a compliance scanner. Every rule prevents a specific high-impact failure: production namespace deletion, world-open security groups, unencrypted S3 buckets, wildcard IAM policies, privileged containers.

Design principles:

- **Catastrophic only** — no hygiene rules, no best-practice noise.
- **Deterministic** — evaluated from static configuration without runtime API calls.
- **Low false-positive rate** — every rule maps to a known attack chain or incident class.

See [docs/POLICY_CATALOG.md](docs/POLICY_CATALOG.md) for the full rule catalog.

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

Apache License 2.0
