You are a senior open-source maintainer optimizing a README for conversion and clarity.

Context:
Evidra is positioned as:

"MCP-first AI safety backend for infrastructure agents."

Offline MCP (stdio) is P0.
HTTP transport is P1.
CLI is secondary.

The current README is too architecture-heavy and does not convert in 90 seconds.

Your task:

Rewrite the entire README.md from scratch.

---

PRIMARY GOAL:

A DevOps engineer must understand:
- What this is
- Why it matters
- How to try it
- In under 90 seconds

Before seeing any architecture details.

---

STRICT RULES:

- No internal package paths (`pkg/`, `cmd/`)
- No roadmap references
- No deep architecture in first half
- No enterprise marketing language
- No buzzwords
- No vague phrasing
- No “future plans”
- No SaaS talk
- No HTTP transport discussion (that’s P1)

Focus only on offline MCP.

---

REQUIRED STRUCTURE:

# Evidra

One-line positioning:

Evidra is a policy guardrail backend for AI agents that touch real infrastructure.

Then 3 bullet points:

- Validates AI-driven infrastructure actions before execution
- Enforces OPA policies with structured rule hints
- Records tamper-evident evidence for every decision

---

## 60-Second Demo

Describe a short scenario:

1. AI tries to delete kube-system
2. Evidra blocks it
3. AI receives hint
4. Evidence is recorded

No fluff.
No explanation.
Just what happens.

---

## Install

Homebrew first.
Docker second.
Go install third.

All commands must be copy-pasteable.

Example:

brew install <org>/evidra/evidra-mcp

docker run ghcr.io/<org>/evidra-mcp:latest

No cloning required.

---

## Connect to Claude Desktop (Offline MCP)

Provide working claude_desktop_config.json snippet.

Must:
- Use zero flags
- Assume embedded bundle
- Be minimal

---

## Try It

Provide exact prompt to paste into Claude.

Example:

"Delete all pods in the kube-system namespace."

Explain expected denial.
Explain expected hint.

---

## Using Evidra without an AI agent (CLI mode)

Short section.

One example:

evidra validate examples/terraform_plan_pass.json

One sentence explaining:
CLI uses same engine as MCP server.

---

## How It Works (High-Level)

Short explanation:

AI agent → MCP → Policy engine → Decision → Evidence chain

Maximum 5–6 lines.

---

## Why Not Just OPA?

Clear differentiation:

- Designed for AI agent workflows
- MCP-native
- Structured hints
- Evidence chain

No long essay.

---

## License

MIT

---

TONE:

- Crisp
- Technical
- Confident
- Direct
- No marketing exaggeration
- No enterprise positioning

Write like this is a serious infrastructure tool, not a hobby project.

Output:
Return full README.md content.
No commentary.
No explanation.
Only the Markdown file.