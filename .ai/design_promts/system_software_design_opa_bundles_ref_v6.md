You are acting as a Principal Software Architect performing a MICRO-HARDENING PASS.

Goal:
Apply only the specific clarifications identified in the latest review (REM-1..REM-4),
without changing architecture, scope, or introducing new features.

Documents to update in-place:
1) AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
2) AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md

Return full updated documents (no diffs).

Constraints:
- No redesign
- No new components
- No SaaS
- No MCP
- No multi-bundle
- No code
- No pseudo-code
- Keep strict MUST/MUST NOT language

------------------------------------------------------------
CHANGES TO APPLY (ONLY THESE)
------------------------------------------------------------

REM-1 — Fix PolicyRef integrity wording (System Design doc)
- In the "PolicyRef / BundleRevision" authority table (or equivalent section),
  remove or correct any statement implying that PolicyRef provides content integrity in BundleSource mode.
- Clarify explicitly:
  - In bundle mode, content integrity is provided by the release artifact checksum + build/release pipeline.
  - PolicyRef() is an opaque identifier and MUST NOT be used for integrity/provenance decisions when BundleRevision is present.
  - For LocalFileSource, PolicyRef may be a content hash, but it is informational in bundle mode.

REM-2 — Name the mandated param helper (System Design + Guardrails)
- Wherever the docs mandate a shared helper for parameter resolution, make it concrete:
  - Define the helper name: `resolve_param`
  - Define the canonical location file: `evidra/policy/defaults.rego`
  - Define the canonical package: `evidra.policy` (or the existing top-level policy package used in the doc)
  - Define its responsibilities in words (no code): env lookup + by_env fallback chain + unresolved handling.
- Ensure Guardrails references the same helper name and location.

REM-3 — Tighten INV-2 enforcement scope (Guardrails)
- INV-2 currently lists open-ended examples of "infrastructure-specific identifiers".
  Add one sentence clarifying:
  - CI enforces only the explicit, documented subset of patterns/checks.
  - Any additional identifier patterns are subject to code review until explicitly added to CI.
- Do NOT expand CI rules; just clarify enforcement scope.

REM-4 — Make determinism test iteration count fixed (System Design + Guardrails if referenced)
- Replace any range like "N=50-100" with a single fixed value:
  - Set N = 50 (unless the document already standardizes another value).
- Ensure all references use the same fixed N.

------------------------------------------------------------
OUTPUT REQUIREMENTS
------------------------------------------------------------

- Update only the relevant paragraphs/sections.
- Do not rewrite unrelated sections.
- Keep documents internally consistent.
- Return:
  1) Updated AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md (full text)
  2) Updated AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md (full text)