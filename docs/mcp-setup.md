# Evidra MCP — Setup & Usage Guide

Evidra is an infrastructure safety gate. It validates planned changes (Kubernetes, Terraform, Helm, AWS, ArgoCD) against OPA policies before execution and produces a signed evidence trail.

This guide covers connecting Evidra MCP server to your AI agent and getting the most out of it.

---

## Quick Start

### 1. Install

```bash
# Option A: Homebrew
brew install samebits/tap/evidra-mcp

# Option B: Go
go install github.com/vitas/evidra/cmd/evidra-mcp@latest

# Option C: From source
git clone https://github.com/vitas/evidra.git
cd evidra && go build -o evidra-mcp ./cmd/evidra-mcp
```

### 2. Connect to your agent

**Claude Code:**
```bash
claude mcp add evidra -- evidra-mcp --bundle ops-v0.1
```

**Cursor / Claude Desktop / Windsurf (JSON config):**
```json
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": ["--bundle", "ops-v0.1"]
    }
  }
}
```

**Codex:**
```bash
# In codex MCP config (mcp.json or codex config):
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": ["--bundle", "ops-v0.1"]
    }
  }
}
```

**Gemini CLI:**
```json
// In ~/.gemini/settings.json under mcpServers:
{
  "evidra": {
    "command": "evidra-mcp",
    "args": ["--bundle", "ops-v0.1"]
  }
}
```

**OpenClaw:**
```json
// In ~/.openclaw/openclaw.json — MCP section:
{
  "mcp": {
    "servers": {
      "evidra": {
        "command": "evidra-mcp",
        "args": ["--bundle", "ops-v0.1"],
        "transport": "stdio"
      }
    }
  }
}
```

Or in agent config (openclaw.yaml for Clawctl):

```yaml
agents:
  - id: main
    model: anthropic/claude-sonnet-4-5
    mcp_servers:
      - name: evidra
        command: evidra-mcp
        args: ["--bundle", "ops-v0.1"]
```

OpenClaw agents need explicit instructions to use Evidra. Add the system prompt block from the [Other Agents](#codex--gemini-cli--other-agents) section to your agent's `systemPrompt` in openclaw.json:

```json
{
  "agents": {
    "defaults": {
      "systemPrompt": "... your prompt ...\n\n## Infrastructure Safety Gate (Evidra)\n\nYou have access to an MCP tool called \"validate\". You MUST call it before executing any command that creates, modifies, or deletes infrastructure resources.\n\nWhen to call: kubectl apply/delete/create/patch, terraform apply/destroy, helm install/upgrade/uninstall, argocd sync, AWS mutations.\n\nSkip for: get, describe, list, plan, show, diff, status.\n\nCRITICAL: If validate returns allow=false, DO NOT execute the command. Show the deny reasons and hints to the user. If validate fails or is unreachable, DO NOT execute — fail closed."
    }
  }
}
```

**Hosted endpoint (no local install):**
```json
{
  "mcpServers": {
    "evidra": {
      "url": "https://evidra.samebits.com/mcp",
      "headers": { "Authorization": "Bearer ev1_YOUR_KEY" }
    }
  }
}
```

### 3. Test

Ask your agent: *"What tools do you have from Evidra?"*

You should see two tools: `validate` and `get_event`.

---

## How It Works

Evidra exposes two MCP tools:

**`validate`** — Check a planned infrastructure change against policy. Returns `allow` or `deny` with reasons, hints, and an evidence ID.

**`get_event`** — Retrieve a previous evidence record by ID for audit.

### The workflow

```
You → Agent: "Deploy nginx to production"
       Agent → Evidra: validate(kubectl, apply, namespace=production, ...)
       Evidra → Agent: deny — missing resource limits, mutable image tag
Agent → You: "Blocked: resource limits required, use a pinned image tag"
```

When allowed:
```
You → Agent: "Deploy nginx to default namespace with limits and pinned tag"
       Agent → Evidra: validate(kubectl, apply, namespace=default, ...)
       Evidra → Agent: allow — evidence evt-abc123
Agent → You: "Approved (evt-abc123), deploying now"
       Agent → executes kubectl apply
```

---

## What Evidra Checks (ops-v0.1 bundle)

The default policy bundle covers:

**Kubernetes:**
- Protected namespaces (kube-system, kube-public) — modifications denied
- Privileged containers — runAsRoot, privileged mode, host namespaces
- Dangerous capabilities — SYS_ADMIN, NET_ADMIN, SYS_PTRACE
- Mass deletes — `--all` or broad selectors
- Missing resource limits — no CPU/memory on containers
- Mutable image tags — `:latest` or no tag in production
- HostPath mounts — filesystem access from containers
- Public exposure — LoadBalancer/NodePort without approval

**Terraform:**
- IAM wildcard actions or principals
- S3 public access
- S3 without encryption or versioning
- Security groups open to 0.0.0.0/0

**ArgoCD:**
- Auto-sync enabled (unsafe for production)
- Wildcard cluster destinations
- Dangerous sync options

**General:**
- Production changes without change-approved tag
- Autonomous execution warnings (agent acting without human)
- Breakglass usage tracking
- Insufficient context for evaluation

---

## Agent Instructions

This section explains how your AI agent should use Evidra. If you're using Claude with the Evidra Skill installed, this happens automatically. For other agents (Codex, Gemini CLI), include these instructions in your system prompt or agent configuration.

### When to validate

**Always validate before:** `kubectl apply/delete/create/patch`, `terraform apply/destroy`, `helm install/upgrade/uninstall`, `argocd app sync`, AWS mutations (`s3 rm`, `ec2 terminate-instances`, `iam create-*`).

**Skip for read-only:** `get`, `describe`, `list`, `plan`, `show`, `diff`, `status`.

### Fail-closed rule

**If validation returns deny, OR fails with an error, OR the validate tool cannot be reached — DO NOT proceed with the mutation.** Show the user what happened and wait for guidance.

### How to call validate

```json
{
  "actor": {
    "type": "agent",
    "id": "claude",
    "origin": "mcp"
  },
  "tool": "kubectl",
  "operation": "apply",
  "params": {
    "target": {
      "namespace": "production"
    },
    "payload": {
      "containers": [{
        "image": "nginx:1.27",
        "resources": {"limits": {"cpu": "500m", "memory": "256Mi"}}
      }]
    },
    "risk_tags": []
  },
  "context": {
    "environment": "prod",
    "dry_run": false,
    "source": "mcp"
  }
}
```

**Required fields:** `actor`, `tool`, `operation`, `params`, `context`.

**Filling params from context:**
- **kubectl apply -f manifest.yaml** → read the YAML, extract namespace, containers (image, resources, securityContext), volumes, service type
- **terraform apply** → extract resource counts (create/update/delete), resource types from plan JSON
- **helm upgrade** → chart name, release name, namespace, overridden values
- **kubectl delete** → resource type, name or selector, namespace

If context is sparse, call validate with what you have. More context = better policy matching. A minimal call with just tool + operation + namespace is still valuable.

### Handling responses

**allow=true:** Proceed. Optionally note the evidence ID for audit.

**allow=false:** Stop. Show the user: reasons (human-readable), rule IDs, risk level, hints (how to fix). Never retry with the same parameters.

**Error (ok=false with error field):** Evidra itself had a problem. Do not proceed. Tell the user validation could not complete.

### Evidence retrieval

Every validation creates an evidence record. Use `get_event` with the `event_id` to retrieve it. Useful when the user asks "why was that deploy blocked?" or needs an audit reference.

---

## Claude Skill (automatic integration)

For Claude users, we provide a Skill that automates everything above. The skill teaches Claude when to validate, how to assemble parameters, and how to handle deny responses — no manual prompting needed.

### Install the skill

Canonical repo path: `skills/evidra-infra-safety/`.

1. Download `evidra-infra-safety/` from [GitHub](https://github.com/vitas/evidra/tree/main/skills/evidra-infra-safety)
2. Zip the folder
3. Upload to Claude.ai → Settings → Capabilities → Skills
4. Or place in Claude Code skills directory

### What the skill does

- Automatically triggers on infrastructure mutation requests
- Extracts parameters from manifests, plans, and commands
- Calls `validate` before execution
- Stops on deny with clear reasons and fix suggestions
- Skips read-only operations

### Verify

Ask Claude: *"Delete all pods in kube-system"*

Expected: Claude calls validate → gets denied (protected namespace) → shows reasons and hints → does not execute.

---

## Codex / Gemini CLI / OpenClaw / Other Agents

Agents without Claude Skill support need explicit instructions. Add the following to your agent's system prompt or instructions file (for OpenClaw, add it to `systemPrompt` in openclaw.json — see [connection config above](#2-connect-to-your-agent)):

```
## Infrastructure Safety Gate (Evidra)

You have access to an MCP tool called "validate". You MUST call it
before executing any command that creates, modifies, or deletes
infrastructure resources.

When to call: kubectl apply/delete/create/patch, terraform apply/destroy,
helm install/upgrade/uninstall, argocd sync, AWS mutations.

Skip for: get, describe, list, plan, show, diff, status.

How to call: provide tool name, operation, and params with target
namespace and relevant payload details from the manifest/plan.

CRITICAL: If validate returns allow=false, DO NOT execute the command.
Show the deny reasons and hints to the user. If validate fails with
an error or is unreachable, DO NOT execute — fail closed.

After a successful validation, proceed with execution and note the
evidence ID (event_id) for audit trail.
```

This prompt block works with any MCP-compatible agent. Adjust `actor.id` to match your agent name.

---

## Configuration

### Policy bundles

Default bundle: `ops-v0.1` — covers Kubernetes, Terraform, ArgoCD, AWS.
Ships embedded in the binary; extracted automatically on first run (zero-config).

```bash
# Explicit bundle path (optional — embedded ops-v0.1 used by default)
evidra-mcp --bundle ./policy/bundles/ops-v0.1
```

Legacy loose-file mode (individual `.rego` + `data.json`):
```bash
evidra-mcp --policy ./my-policy.rego --data ./my-data.json
```

### Evidence storage

Default: `~/.evidra/evidence`

Override:
```bash
evidra-mcp --evidence-store /var/lib/evidra/evidence
```

### Environment

Set the environment for policy evaluation:
```bash
evidra-mcp --environment production
```

This affects environment-dependent rules (e.g. "production requires change-approved tag",
"S3 versioning required in production"). Values are normalized: `prod` → `production`,
`stg` → `staging`.

### Connection modes

```bash
# Local-only (default) — embedded bundle, no network
evidra-mcp

# Online — evaluations sent to API server
EVIDRA_URL=https://api.evidra.rest EVIDRA_API_KEY=ev1_... evidra-mcp

# Online with offline fallback — use API, fall back to local if unreachable
EVIDRA_URL=https://api.evidra.rest EVIDRA_API_KEY=ev1_... evidra-mcp --fallback-offline

# Force offline — skip API even if EVIDRA_URL is set
evidra-mcp --offline
```

### Environment variables

| Variable | Description |
|---|---|
| `EVIDRA_URL` | API endpoint (enables online mode) |
| `EVIDRA_API_KEY` | Bearer token (required when `EVIDRA_URL` is set) |
| `EVIDRA_ENVIRONMENT` | Environment label (`prod`, `staging`, `development`) |
| `EVIDRA_BUNDLE_PATH` | OPA bundle directory (alternative to `--bundle` flag) |
| `EVIDRA_FALLBACK` | `closed` (default) or `offline` |
| `EVIDRA_EVIDENCE_DIR` | Evidence store directory (alternative to `--evidence-store` flag) |
| `EVIDRA_POLICY_PATH` | Rego file path (legacy, alternative to `--policy` flag) |
| `EVIDRA_DATA_PATH` | Data JSON path (legacy, alternative to `--data` flag) |

---

## Troubleshooting

**Agent doesn't call validate:**
- Verify MCP connection: ask the agent "what tools do you have?"
- If validate not listed: check MCP config, restart agent
- If listed but not called: add explicit instructions (see Codex/Gemini section)
- For Claude: install the Evidra Skill for automatic triggering

**Validation returns error instead of allow/deny:**
- Check policy bundle path: `--bundle ops-v0.1` must resolve to actual files
- Check evidence directory is writable
- Run `evidra-mcp` standalone to verify: `echo '{}' | evidra-mcp` should start without errors

**All validations return allow:**
- Verify policy bundle has rules for your tool/operation
- Check if the params provide enough context for rules to match
- Ensure `--environment` matches the rules (some rules only apply in `production`)

**Evidence not found:**
- Default store: `~/.evidra/evidence`
- Check if overridden via `--evidence-store`
- Evidence files are append-only; check disk space
