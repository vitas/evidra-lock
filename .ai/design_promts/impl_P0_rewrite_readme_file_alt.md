You are a senior open-source maintainer optimizing a README for conversion.

Context:
Evidra is now positioned as:

"MCP-first AI guardrail backend for infrastructure agents."

The MCP server (`evidra-mcp`) is the primary product.
The CLI is secondary and must appear later.

Goal:
Rewrite the entire README.md in MCP-first format.

---

CRITICAL RULES:

- The README must convert in 90 seconds.
- It must start with a strong positioning sentence.
- It must show value BEFORE architecture.
- It must prioritize MCP usage.
- CLI must be mentioned only after MCP.
- No internal package paths.
- No deep architecture early.
- No walls of text.

---

STRUCTURE REQUIRED:

# Evidra

One-sentence positioning line.

Short 3-bullet explanation:
- What it does
- Who it's for
- Why it matters

---

## 60-Second Demo

Show:

1. AI tries dangerous action
2. Evidra blocks it
3. Evidence chain recorded

Describe demo clearly.

---

## Install (MCP-first)

Homebrew first.
Docker second.
`go install` third.

All commands must be copy-pasteable.

---

## Connect to Claude (Offline MCP)

Provide working `claude_desktop_config.json` snippet.

Must be minimal.
Must work with zero flags (embedded bundle).

---

## Try It

Explain exact prompt to send to Claude.

Explain expected denial.

---

## Using Evidra without an AI agent (CLI mode)

Explain briefly:
- CLI shares same engine
- Good for CI or local validation
- Provide one simple CLI example

Keep it short.

---

## How It Works (High-Level)

Short explanation:
Scenario → OPA → Decision → Evidence chain.

No deep internals.

---

## Why Not Just OPA?

Explain differentiation clearly:
- AI-agent focused
- Structured MCP integration
- Tamper-evident evidence chain

---

## License

---

Tone:

- Crisp
- Confident
- No fluff
- No enterprise buzzwords
- No future roadmap talk

Write as if trying to get GitHub stars and Hacker News interest.

---

Output:
Return full README.md content.
Do not include commentary outside the file.