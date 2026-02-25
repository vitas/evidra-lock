# Evidra

Evidra is a policy guardrail backend for AI agents that touch real infrastructure.

- Validates AI-driven infrastructure actions before execution
- Enforces OPA policies with structured rule hints
- Records tamper-evident evidence for every decision

---

## 60-Second Demo

**Scenario:** An AI agent attempts to delete all pods in `kube-system`.

1. Agent calls the `validate` MCP tool with the intended action.
2. Evidra evaluates it against policy. `kube-system` is a protected namespace.
3. Agent receives `allow: false`, `risk_level: "high"`, and a hint:
   > `Add risk_tag: change-approved to bypass with explicit approval`
4. The decision is recorded to the local evidence chain with a unique `event_id`.

The agent stops. Nothing is deleted. The audit trail exists.

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

No extra flags required — the binary ships with the ops-v0.1 policy bundle built in.
For other MCP clients, use the same `command` / `args` shape in their respective config.

---

## Try It

Open Claude Desktop or Claude Code and paste this prompt:

> "Delete all pods in the kube-system namespace."

**What you'll see:**

Claude will attempt to call `validate` before acting. Evidra will block the action and return:

- `allow: false`
- `risk_level: "high"`
- Rule ID: `k8s.protected_namespace`
- Hint: how to properly tag the action if the change is authorized

Claude cannot proceed. The block and the hint are both recorded in the evidence chain.

---

## CLI Mode

Run Evidra without an AI agent to validate plan files directly:

```bash
evidra validate examples/terraform_plan_pass.json
```

Add `--explain` for rule hits and hints, `--json` for structured output.

The CLI uses the same evaluation engine as the MCP server. Output is identical.

---

## How It Works

```
AI agent → MCP validate tool → Policy engine (OPA) → Decision → Evidence chain
```

The MCP server receives a `ToolInvocation`, evaluates it against the active OPA bundle, and returns a structured decision (`allow`, `risk_level`, `reasons`, `hints`). Every decision — allow or deny — is written as an append-only, hash-linked record under `~/.evidra/evidence`.

Decisions include rule IDs and actionable hints so the agent can either stop, retry with correct tags, or escalate.

---

## Why Not Just OPA?

OPA evaluates policy. Evidra evaluates AI agent behavior.

| | OPA | Evidra |
|---|---|---|
| Input model | JSON documents | AI `ToolInvocation` schema |
| Transport | HTTP / GRPC | MCP stdio (native agent integration) |
| Structured hints | No | Yes — per rule, returned to agent |
| Evidence chain | No | Yes — append-only, hash-linked JSONL |
| Agent-aware | No | Yes — actor, intent, risk tags |

---

## License

Apache License 2.0
