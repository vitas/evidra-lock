You are doing a THINK-FIRST redesign of rule identifiers for the Evidra project BEFORE any refactor.

Goal:
Replace legacy semantic IDs like `POL-PROD-01`, `WARN-AUTO-01` with a clean, stable, future-proof rule_id scheme.

We will NOT implement changes yet.
First we will design the rule_id system and produce a complete mapping proposal.

OUTPUT FILE (MANDATORY):
Create the document:

.ai/AI_CLAUDE_RULE_ID_REDESIGN_SPEC.md

All output must be written into this file only.

Chosen canonical style (must use this):
- `domain.invariant_name`
  - lowercase
  - dot-separated domain
  - snake_case invariant

Examples:
- terraform.mass_delete
- k8s.privileged_container
- argocd.autosync_enabled
- s3.public_access

Hard constraints:
- rule_id MUST NOT encode environment (prod/dev/stage/etc.)
- rule_id MUST NOT encode severity (warn/deny/allow/etc.)
- rule_id MUST NOT include ordinals (01/02/03)
- rule_id MUST be stable across versions
- rule_id MUST describe the violated invariant (what is wrong), not context

------------------------------------------------------------
REQUIRED CONTENT OF ai/AI_RULE_ID_REDESIGN_SPEC.md
------------------------------------------------------------

Section 1 — Rule ID Naming Standard
- Final normative standard text
- MUST/MUST NOT language
- Clear formatting rules
- Examples of valid and invalid rule IDs

Section 2 — Allowed Domains (Taxonomy)
- Controlled vocabulary of domains (e.g., terraform, k8s, argocd, helm, s3, iam, ops)
- Short description of what belongs in each domain
- Rules for introducing a new domain

Section 3 — Complete Mapping Table
- Markdown table:
  | old_id | new_rule_id | rationale |
- Every existing rule must be mapped
- No duplicates
- No collisions
- If any ambiguity exists, document it explicitly

Section 4 — Consistency Checklist
- Collision check
- Naming clarity check
- Invariant stability check
- Scope correctness check
- Domain correctness check

Section 5 — Decision Log
- Why `domain.invariant_name` was chosen
- Why environment/severity removed from IDs
- Tradeoffs considered
- Why this is future-proof

------------------------------------------------------------
PROCESS REQUIREMENTS
------------------------------------------------------------

- Enumerate all existing rule IDs in:
  - Rego files
  - Go constants
  - Tests
  - Docs
- Group them by invariant meaning.
- Propose new rule IDs following the canonical style.
- Ensure zero duplication.
- Ensure semantic clarity.
- Ensure future extensibility.

Do NOT:
- Write implementation code
- Provide diffs
- Introduce migration/compatibility layer
- Modify architecture
- Introduce SaaS or multi-bundle

This is a design-only specification.