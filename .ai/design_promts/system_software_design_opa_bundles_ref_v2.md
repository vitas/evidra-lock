You are acting as a Principal Software Architect performing a SECOND HARDENING PASS.

Objective:
Strengthen the existing architecture documents by tightening determinism guarantees,
policy identity rules, and ambiguity points identified in review.

Do NOT redesign.
Do NOT introduce new components.
Do NOT introduce SaaS.
Do NOT introduce multi-bundle.
Do NOT add new features.

This is a precision hardening pass only.

Documents to refine:

1) AI_SYSTEM_DESIGN_OPA_BUNDLES_CLAUDE.md
2) AI_ARCHITECTURE_GUARDRAILS_CLAUDE.md
3) AI_BUNDLE_BUILD_AND_RELEASE_STRATEGY_CLAUDE.md

------------------------------------------------------------
REQUIRED REFINEMENTS
------------------------------------------------------------

### 1) Explicit Unknown Environment Contract

In System Design:

- Add a MUST-level invariant:
  Every bundle MUST explicitly define behavior for unknown environments.
- Clarify that undefined environment lookup is deterministic OPA behavior,
  but bundle author must choose fail-open or fail-closed intentionally.
- State that absence of explicit unknown-environment rule is a policy design decision,
  not an engine decision.

This must be formal and unambiguous.

------------------------------------------------------------

### 2) Policy Identity Authority Rule

In System Design and Guardrails:

Add a strict rule:

- When BundleRevision is non-empty,
  PolicyRef MUST NOT be used as policy identity for:
    - replay
    - audit
    - provenance
    - release lookup

- BundleRevision is authoritative identity.
- PolicyRef is informational only in bundle mode.

Make this a non-optional invariant.

------------------------------------------------------------

### 3) Canonical JSON Serialization Requirement

In Determinism section:

- Specify that decision serialization must use canonical JSON:
  - stable key ordering
  - no insignificant whitespace differences
  - no map-order leakage
  - consistent floating-point representation

- State that determinism tests must use canonical encoding.

Make this a release-blocking requirement.

------------------------------------------------------------

### 4) ProfileName Derivation Rule

Remove ambiguity around profile derivation.

Define ONE authoritative source for ProfileName:

Either:
- Derived strictly from manifest field
OR
- Derived strictly from artifact filename by documented rule

Pick one and make it mandatory.

No dual derivation paths.

------------------------------------------------------------

### 5) AST-Based Guardrail Preference

In Guardrails:

- Clarify that regex checks are fallback mechanisms.
- Where feasible, static analysis must operate on parsed AST.
- False-positive suppression must be narrow and documented.

Add explicit warning that regex-only enforcement is insufficient long-term.

------------------------------------------------------------

### 6) Manifest Injection Integrity Rule

In Build & Release Strategy:

Add:

- CI must verify repository `.manifest` does not contain production revision.
- Manifest revision must be injected from Git tag.
- Repository must not store release revision values permanently.
- Tag, manifest revision, and artifact filename must match exactly.

------------------------------------------------------------

### 7) Deterministic gzip Clarification

In Build & Release Strategy:

- Clarify that deterministic gzip must use:
  - fixed mtime
  - no filename in header
  - OS field normalized
- Recommend using a single controlled implementation (e.g., Go stdlib).

Avoid implying cross-implementation determinism guarantees.

------------------------------------------------------------

OUTPUT FORMAT
------------------------------------------------------------

For each document:

1) Provide strengthened replacement sections only (do not rewrite entire document).
2) Label each change with:
   - "HARDENING ADDITION"
   - "CLARIFICATION"
   - or "INVARIANT STRENGTHENING"
3) Keep strict architectural tone.
4) No code.
5) No pseudo-code.
6) No marketing language.

Goal:
Upgrade architecture from "very strong" to "audit-grade deterministic specification".