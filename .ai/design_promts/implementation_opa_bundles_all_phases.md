You are operating in a STRICT FOUR-PHASE MODE.

This refactor MUST be executed in four completely separate phases:

PHASE 1 — Conversion Design & Planning (NO CODE OR DOC CHANGES)
PHASE 2 — Implementation & Refactor (Code Changes Allowed ONLY)
PHASE 3 — Documentation & Public Surface Update (Docs Changes Allowed ONLY)
PHASE 4 — Repo-Wide Consistency & Drift Sweep (Automated checks + cleanups)

You MUST complete each phase fully and wait for explicit approval before proceeding.

Fail-fast rule:
- Any code edits in Phase 1 or Phase 3 = failure
- Any doc edits in Phase 2 or Phase 4 = failure (Phase 4 is checks + minimal cleanups only as defined)
- Starting a phase without explicit approval = failure

====================================================================
CONTEXT
====================================================================

We are refactoring an existing Evidra repository to comply with:

- AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
- AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md

Constraints include:
- OPA-native bundle layout
- directory-based data.json loading
- explicit PolicySource metadata accessors
- deterministic output (canonical JSON, N=50)
- params namespace consolidation
- single-bundle invariant
- strict guardrails enforcement
- no multi-bundle
- no SaaS
- no MCP
- environment is opaque string
- no semantic branching on PolicyRef()

No backward compatibility required.

====================================================================
PHASE 1 — CONVERSION DESIGN (NO MODIFICATIONS)
====================================================================

Objective:
Produce a complete conversion plan.

You MUST:
1) Inventory current repo (packs, rego, data, rule IDs, docs references)
2) Identify spec violations (OPA-unloadable data files, forbidden namespaces, etc.)
3) Define target bundle structure per profile
4) Define namespace + rule ID migrations
5) Define ordered atomic steps
6) Define validation checklist
7) Risk assessment

Output format:
- Current State Inventory
- Architectural Gaps
- Target Bundle Layout(s)
- Namespace & Rule ID Migration Map
- Ordered Refactor Plan
- Validation Checklist
- Risk Assessment

STOP after Phase 1.
WAIT for approval:
"APPROVE PHASE 1 — PROCEED TO IMPLEMENTATION"

====================================================================
PHASE 2 — IMPLEMENTATION & REFACTOR (CODE ONLY)
====================================================================

Objective:
Execute Phase 1 plan exactly (code changes only).

Deliverables:
- Updated bundle structure
- Updated loader/evaluator wiring
- Updated PolicySource metadata accessors
- Deterministic output enforcement
- CI guardrails implemented
- List of modified files
- Acceptance checklist vs Phase 1

STOP after Phase 2.
WAIT for approval:
"APPROVE PHASE 2 — UPDATE DOCUMENTATION"

====================================================================
PHASE 3 — DOCUMENTATION UPDATE (DOCS ONLY)
====================================================================

Objective:
Update all public/internal docs to match new architecture (docs changes only).

Update:
- README(s)
- docs/ guides
- examples
- CI docs
- remove legacy terminology (thresholds/environments)
- update bundle layout examples and --profile usage

Deliverables:
- List of updated doc files
- Summary per file
- Final doc consistency checklist

STOP after Phase 3.
WAIT for approval:
"APPROVE PHASE 3 — RUN CONSISTENCY SWEEP"

====================================================================
PHASE 4 — REPO-WIDE CONSISTENCY & DRIFT SWEEP
(AUTOMATED CHECKS + MINIMAL CLEANUPS)
====================================================================

Objective:
Prove the repo is internally consistent and no legacy artifacts remain.
This phase is primarily verification; only minimal cleanups are allowed.

Allowed changes in Phase 4:
- Remove unused/obsolete files left behind by refactor
- Fix broken links in docs caused by renames
- Fix minor path typos in examples/tests
- Update CI grep patterns if they are too broad/narrow (must match guardrails intent)
NO new features, no architectural changes.

Phase 4 Checklist (must execute and report results):

A) Terminology Sweep (repo-wide)
- Confirm zero remaining references to legacy concepts/paths:
  - "thresholds"
  - "environments"
  - old pack paths that no longer exist
- Confirm no references to non-OPA-loadable data files (e.g., params.json outside data.json structure)

B) Bundle Structure Validation
- For each bundle under policies/bundles/*:
  - .manifest exists at bundle root
  - roots == ["evidra"] (MVP)
  - metadata.profile_name present and matches directory name (per spec)
  - OPA-loadable data docs exist under evidra/data/**/data.json

C) Guardrails Verification (static)
- Confirm no env string literal comparisons in Rego
- Confirm no forbidden namespaces:
  - data.evidra.data.thresholds
  - data.evidra.data.environments
- Confirm Engine has no PolicyRef semantic branching:
  - no prefix checks, regex checks, length inference on PolicyRef
- Confirm Engine does not type-assert PolicySource concrete types for metadata

D) Determinism Verification (runtime)
- Run determinism test N=50 and report pass/fail
- Confirm stable sorting of violations and canonical JSON byte identity

E) Documentation Link/Example Verification
- Validate all example bundle trees in docs match actual repo layout
- Validate all command examples use current flags and paths
- Validate README has no stale references

Phase 4 Output (must include):
- A pass/fail table for A–E
- Exact list of remaining issues (if any)
- Exact list of minimal cleanups performed
- Final “Repo is consistent” statement only if all checks pass

STOP after Phase 4.
Do not proceed further.

====================================================================
FAIL CONDITIONS
====================================================================

- Editing code in Phase 1 or Phase 3
- Editing docs in Phase 2
- Starting a phase without explicit approval
- Introducing multi-bundle logic
- Adding environment enums
- Weakening determinism requirements
- Introducing PolicyRef semantic branching

====================================================================
GOAL
====================================================================

Deliver a clean, deterministic, OPA-native, single-bundle-compliant repository
with aligned code, CI, and documentation, and a verified drift-free final state.

