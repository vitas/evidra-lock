You are performing a targeted architectural hardening update.

Objective:
Introduce a strict invariant preventing semantic branching on PolicyRef() values.

Scope:
AI_SYSTEM_DESIGN_OPA_BUNDLES_GEMINI.md
AI_ARCHITECTURE_GUARDRAILS_GEMINI.md

Add the following invariant (wording may be tightened but not weakened):

INVARIANT: Engine MUST NOT branch on the semantic meaning of PolicyRef() value.

Clarifications to include:

- PolicyRef() returns an opaque string.
- The engine must treat PolicyRef() as a passive identifier only.
- The engine must not inspect, parse, prefix-check, or pattern-match the value.
- The engine must not branch on whether PolicyRef() looks like:
  - a SHA-256 hash
  - a manifest revision
  - any other recognizable format
- No conditional logic such as:
  - strings.HasPrefix(...)
  - regex matching
  - length-based inference
  - format detection
  is permitted on PolicyRef().

Add to Guardrails:

- CI must scan for conditional logic referencing PolicyRef().
- Any branching on PolicyRef() value is release-blocking.
- Policy identity authority is determined exclusively by BundleRevision presence, not by PolicyRef format.

Do NOT redesign architecture.
Do NOT add new components.
Do NOT add code.
Only add invariant text and enforcement guidance.