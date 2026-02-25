You are a principal AI infrastructure architect.

I have a Go-based project called "Evidra".
It currently provides:
- Policy validation via OPA bundles
- Evidence chain logging (hash-linked JSONL segments)
- CLI validator
- MCP server integration
- Observe mode (non-blocking advisory)
- Designed for AI-assisted infrastructure operations

I want to transform this into a FULL AI SKILLS BACKEND.

The goal:
Evidra should become the execution control plane for AI agents that perform infrastructure actions (Terraform, Kubernetes, cloud APIs, shell commands, etc).

It should:
- Act as policy-enforced execution gateway
- Provide structured skill execution
- Record tamper-evident evidence
- Support multi-agent environments
- Be composable and extensible
- Potentially evolve into a SaaS

IMPORTANT OUTPUT REQUIREMENT:

You MUST produce the final result as a complete Markdown document.

The document:
- Must be production-quality
- Must be deeply technical
- Must be structured and readable
- Must include diagrams in Markdown (ASCII or Mermaid where helpful)
- Must include JSON examples where relevant
- Must be opinionated and concrete
- Must be written as if it will be committed to the repository

The filename MUST be:

AI_CLAUDE_SYSTEM_DESIN_SKILLSET_BACKEND.md

The first line of the output MUST be:

# AI_CLAUDE_SYSTEM_DESIN_SKILLSET_BACKEND

Do not include explanations outside the document.
Do not include meta commentary.
Only output the Markdown document.

---

# 1. Define What an "AI Skills Backend" Must Be

Explain:
- Core capabilities required
- How it differs from a simple MCP server
- Required architectural layers
- Required APIs
- Required runtime guarantees

Be concrete and technical.

---

# 2. Architecture Redesign

Design a production-ready architecture with:

- Skill Registry layer
- Execution sandbox layer
- Policy Enforcement layer
- Evidence Store layer
- Agent Context layer
- Multi-tenant isolation model
- Audit and replay system

For each layer:
- Responsibilities
- Key interfaces
- Failure modes
- Security model

Include architecture diagram (Mermaid preferred).

---

# 3. Skill Model Design

Design a first-class Skill abstraction:

Define:
- Skill metadata schema
- Input/Output contracts
- Versioning strategy
- Deterministic vs non-deterministic classification
- Retry / idempotency model
- Policy attachment model

Include example JSON schemas.

---

# 4. Agent Integration Model

Design:
- How agents discover skills
- How they request execution
- How they receive structured denial + remediation hints
- How they fetch evidence
- How self-healing loops work

Include protocol-level design and example request/response payloads.

---

# 5. Multi-Agent & Multi-Tenant Strategy

Design:
- Isolation boundaries
- Identity model (agent identity, human approval identity)
- Role-based policy mapping
- Per-tenant policy bundles
- Per-tenant evidence chains

---

# 6. Differentiation Strategy

How does this become:
- Better than raw tool-calling
- Better than simple OPA integration
- More valuable than just using cloud-native RBAC

Identify 3 strong differentiators.

---

# 7. Minimal Viable AI Skills Backend (MVP)

Define:
- The smallest possible feature set
- 6-week roadmap
- Must-have vs optional
- What NOT to build

---

# 8. SaaS Evolution Path

If this becomes a hosted control plane:
- Required components
- Billing model ideas
- Enterprise features
- Trust & compliance requirements

---

Be deeply technical.
Avoid generic advice.
Think like someone designing the "Stripe for AI Infrastructure Execution".

Assume the user is highly experienced in Kubernetes, cloud architecture, CI/CD, and backend engineering.