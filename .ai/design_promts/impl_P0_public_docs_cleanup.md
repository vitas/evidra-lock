You are a senior technical writer and OSS maintainer preparing Evidra for public release.

Context:
- Evidra is MCP-first.
- CLI exists but is secondary (policy debugging & evidence tools).
- We are preparing the repository for public visibility.
- Private/internal directories (e.g., .ai/, internal planning docs, strategy notes) will be removed manually.
- We must ensure the public-facing documentation is clean, focused, and credible.

Goals:
1) Create a clean public documentation structure.
2) Ensure positioning is MCP-first.
3) Avoid roadmap-heavy or enterprise-looking docs.
4) Keep project technically serious.
5) Avoid marketing fluff.

STRICT RULES:
- Do NOT change code.
- Do NOT add new features.
- Do NOT introduce SaaS positioning.
- Do NOT add future roadmap promises.
- Do NOT mention internal strategy documents.
- Keep tone technical and calm.

TASKS:

1) Rewrite README.md to be public-ready.

Structure:

# Evidra

One-sentence positioning:
"Evidra is a guardrail layer between AI agents and your infrastructure."

## Why It Exists
Short explanation (AI agents propose infra changes; Evidra validates before execution).

## 30-Second Demo
Simple example (MCP flow first).

## Install
Homebrew
Docker
(go install fallback)

## Connect to Claude Code
Minimal config snippet.

## How It Works
Short technical explanation (MCP → policy engine → decision → hints → evidence).

## Policy Baseline
Brief description:
- Catastrophic guardrails only
- Not a compliance scanner
- Deterministic decisions

Link to:
docs/POLICY_CATALOG.md

## CLI (Policy Debugging & Evidence Tools)
Brief explanation:
- evidra validate
- evidra evidence verify
- evidra evidence export
State clearly:
"The CLI is primarily intended for policy development and debugging."

## License
Apache 2.0

Keep README under ~250 lines.
No roadmap.
No vision document.
No TODO section.

---

2) Restructure docs folder for public clarity.

Create or ensure only these public docs exist:

docs/
  POLICY_CATALOG.md
  ARCHITECTURE.md
  SECURITY_MODEL.md
  CONTRIBUTING.md

Remove references to:
- roadmap
- backlog
- internal strategy
- research scratch notes

ARCHITECTURE.md must:
- Explain MCP-first model
- Explain embedded bundle
- Explain offline design
- Explain evidence model

SECURITY_MODEL.md must:
- Explain deterministic evaluation
- Explain catastrophic-only philosophy
- Clarify non-goals (not CIS, not full compliance)

CONTRIBUTING.md must:
- Explain how to add a rule
- How to run OPA tests
- How to run go test
- Keep short

---

3) Remove private references.

Scan for and remove:
- mentions of .ai/
- internal planning docs
- future roadmap items
- speculative features

Do NOT delete POLICY_CATALOG or serious baseline docs.

---

4) Tone & Positioning Audit

Ensure:
- MCP is primary entrypoint.
- CLI is clearly secondary.
- No phrasing makes it sound like:
  - another conftest
  - another tfsec
  - compliance scanner

Avoid phrases:
- “policy as code tool”
- “security platform”
- “enterprise-grade solution”

Prefer:
- guardrail
- validation layer
- pre-execution control
- deterministic policy engine

---

OUTPUT FORMAT:

Return:
1) Final README.md
2) Final docs folder structure
3) Updated ARCHITECTURE.md
4) Updated SECURITY_MODEL.md
5) Updated CONTRIBUTING.md

No commentary outside the documents.