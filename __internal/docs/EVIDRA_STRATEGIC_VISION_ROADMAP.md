# Evidra --- Strategic Vision & 6--12 Month Roadmap

Generated on: 2026-02-22T19:47:21.535015 UTC

------------------------------------------------------------------------

# 🌍 Long-Term Vision

Evidra becomes the **deterministic safety layer for AI-driven
infrastructure operations**.

Where AI is probabilistic, Evidra is deterministic.

Over time, Evidra should evolve from:

> A validator for AI-generated infra changes

Into:

> The trusted control plane that ensures AI cannot perform unsafe
> infrastructure actions.

But this evolution must be staged carefully.

------------------------------------------------------------------------

# 🎯 Strategic Phases

## Phase 1 --- Sharp Validator (0--3 Months)

**Goal:** Nail product clarity and usability.

Focus:

-   Validate plan and manifest artifacts
-   Deterministic policy (deny/warn)
-   Immutable evidence chain
-   Clean CLI UX
-   Clear AI-readable structured output
-   **Serious Baseline Policy Pack (23 rules)** — 18 must-have guardrails
    sourced from CIS Kubernetes Benchmark (5.2.x), tfsec/trivy (AVD-AWS-*),
    kube-score, AWS S3 best practices, and ArgoCD operational safety patterns.
    Covers: container escape, mass data exposure, irreversible destruction,
    account/cluster compromise, and GitOps safety.
    Research: `docs/backlog/OPS_V0_SERIOUS_BASELINE_RESEARCH.md`

Deliverable:

> A sharp, reliable tool DevOps engineers trust — with enough policy depth
> to be taken seriously on day one.

No enterprise features. No compliance marketing. No workflow automation.

------------------------------------------------------------------------

## Phase 2 --- Policy Intelligence (3--6 Months)

**Goal:** Improve decision quality without increasing complexity.

Focus:

-   Improve outcome parsers for plan and manifest data
-   Smarter risk classification (based on change type, resource
    criticality)
-   Policy modularization (small deny/warn rules)
-   Structured rule labels (stable identifiers)
-   Better AI-readable remediation guidance

Optional (lightweight):

-   Policy simulation mode (what would happen)
-   Local developer feedback loop improvements

Still no compliance suite.

------------------------------------------------------------------------

## Phase 3 --- Trust & Attestation Layer (6--12 Months)

**Goal:** Strengthen evidence and enterprise credibility.

Focus:

-   Cryptographic signing of evidence records
-   External verification CLI (verify evidence chain integrity)
-   Tamper detection improvements
-   Exportable machine-readable evidence format

Optional expansion:

-   Basic CI integration mode
-   Simple GitHub Action
-   Simple API wrapper (minimal)

Not:

-   Full compliance automation
-   Approval workflow engines
-   Governance dashboards

------------------------------------------------------------------------

# 🧠 Product Expansion Principles

Future features must pass this filter:

Does it strengthen deterministic validation of infrastructure outcomes?

If no → reject or postpone.

------------------------------------------------------------------------

# 📊 Strategic Positioning Evolution

### v1 Positioning

> Validate infrastructure changes proposed by AI before they run.

### v2 Positioning

> Deterministic guardrail for AI-driven infrastructure operations.

### v3 Positioning

> Cryptographically verifiable control layer for AI infrastructure
> agents.

------------------------------------------------------------------------

# ⚖️ Avoid These Strategic Traps

-   Becoming a generic DevSecOps platform
-   Building compliance-first messaging too early
-   Creating approval workflow engines
-   Expanding into policy-as-a-service complexity
-   Supporting too many execution modes simultaneously

Complexity is the main long-term risk.

------------------------------------------------------------------------

# 🏁 Long-Term Success Criteria

Evidra succeeds if:

-   DevOps engineers run it locally by default before applying
    AI-generated changes
-   It becomes part of AI-assisted workflows
-   It is trusted for deterministic decisions
-   Its evidence chain becomes a differentiator

------------------------------------------------------------------------

# 🚀 Final Strategic Principle

Keep it sharp. Keep it deterministic. Keep it simple.

Expand only after v1 is trusted and adopted.
