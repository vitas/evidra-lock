# Evidra

Evidra is a guardrail layer between AI agents and your infrastructure.

- 23 OPA policy rules covering Kubernetes, Terraform, ArgoCD, S3, and IAM
- Structured deny responses with rule IDs and actionable hints
- Append-only, hash-linked evidence chain for every decision
- MCP-native: runs as a stdio tool server inside Claude Code, Cursor, or any MCP client

---

## Why It Exists

AI agents can now run `kubectl`, `terraform apply`, and `argocd sync` on production infrastructure. Without a policy layer in the loop, a single misinterpreted prompt can delete a namespace, open a security group to the world, or disable S3 encryption.

Evidra intercepts every tool invocation before it executes. If the action violates policy, the agent receives a structured denial with a rule ID and a hint — not a silent failure. The decision is recorded to a local evidence chain regardless of outcome.

---

## 30-Second MCP Demo

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

5. Claude stops. Nothing is deleted. The denial and the hint are recorded in the evidence chain with a unique `event_id`.

Try another prompt:

> "Create an S3 bucket for storing logs."

If the generated Terraform plan lacks Block Public Access or encryption, Evidra blocks with `terraform.s3_public_access` and `aws_s3.no_encryption` — and tells the agent exactly what to fix.

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

**Binary download**

Grab the latest release from [GitHub Releases](https://github.com/vitas/evidra/releases) and place `evidra-mcp` on your `PATH`.

**Go install**

```bash
go install samebits.com/evidra/cmd/evidra-mcp@latest
```

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

See [docs/POLICY_CATALOG.md](docs/POLICY_CATALOG.md) for the full rule catalog (23 rules across 7 domains).

---

## Try It

Open Claude Code with Evidra connected and try these prompts:

| Prompt | Expected result |
|---|---|
| "Delete all pods in kube-system" | DENY — `k8s.protected_namespace` |
| "Create a public S3 bucket" | DENY — `terraform.s3_public_access`, `aws_s3.no_encryption` |
| "Deploy nginx to the default namespace" | PASS — no rules matched |
| "Open SSH to 0.0.0.0/0" | DENY — `terraform.sg_open_world` |
| "Run a privileged container" | DENY — `k8s.privileged_container` |

Every decision — allow or deny — is written to `~/.evidra/evidence` as an append-only, hash-linked JSONL record.

---

## How It Works

```
AI agent → MCP validate tool → Policy engine (OPA) → Decision → Evidence chain
```

The MCP server receives a `ToolInvocation`, evaluates it against the active OPA bundle, and returns a structured decision (`allow`, `risk_level`, `reasons`, `hints`). Every decision is written as an append-only record under `~/.evidra/evidence`.

Responses include rule IDs and hints so the agent can stop, retry with correct tags, or escalate to a human.

Modes: `enforce` (default — deny blocks the action) or `observe` (`--observe` — policy evaluated and recorded but never blocks).

---

## CLI (Policy Debugging & Evidence Tools)

The `evidra` CLI uses the same evaluation engine as the MCP server. It is intended for local testing, policy development, and evidence inspection.

```bash
# Validate a scenario file against policy
evidra validate examples/demo/kubernetes_kube_system_delete.json
evidra validate --json examples/demo/terraform_public_s3.json
evidra validate --explain examples/demo/kubernetes_safe_apply.json

# Verify evidence chain integrity
evidra evidence verify

# Export evidence for external audit
evidra evidence export --format json
```

Exit codes: `0` = PASS, `2` = FAIL, `1` = error. See [docs/demo-output.md](docs/demo-output.md) for full CLI output examples.

---

## License

Apache License 2.0
