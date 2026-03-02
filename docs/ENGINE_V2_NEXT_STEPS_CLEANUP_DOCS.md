# Evidra Engine v2 — Next Steps (Cleanup + Tests + Docs)

## Scope

This document describes the **next steps after v2 canonicalization is implemented**:
- remove dead code / tests
- keep policy bundle clean and flat-only
- update the “engine logic” documentation to reflect v2 as the new baseline

No API/UI/evidence-store work is included here.

---

## 1) Immediately After Canonicalizer Lands: Stabilize & Verify

### 1.1 Run the full verification set
- `opa test policy/bundles/ops-v0.1/ -v`
- Run MCP stdio/integration tests (if present)
- Run the native-fixture acceptance suite (from the post-canonicalization checklist)

### 1.2 Freeze the invariants in CI
- Forbid `input.actions` in rules/decision/defaults (allow only canonicalize.rego)
- Forbid K8s shape/casing patterns outside canonicalize.rego
- (Optional) Require `defaults.actions` usage explicitly

This prevents the codebase from drifting back into Variant-B style “format branching everywhere”.

---

## 2) Remove Dead Code (Rego)

### 2.1 Identify and delete Variant-B helpers
Once canonicalize.rego is authoritative, remove any helpers that:
- walk K8s shapes (`spec.template.spec`, `metadata.namespace`, `securityContext`, etc.)
- implement casing normalization outside canonicalize.rego

Target file:
- `policy/bundles/ops-v0.1/evidra/policy/defaults.rego`

**Result:** defaults helpers become strictly “flat-only” utilities.

### 2.2 Delete unused format-detection functions
Search and remove helpers or rule fragments that exist solely to cope with:
- multiple payload formats
- missing containers due to nesting
- metadata/namespace fallbacks

After v2, these belong exclusively in canonicalize.rego and should not exist elsewhere.

---

## 3) Remove Dead Code (Go)

If canonicalization is entirely in Rego, confirm there is no leftover Go normalization code:
- any “flattening” helpers that were introduced experimentally
- any special-case translation logic for kubectl payloads

If any exists, delete it and rely solely on the policy canonicalizer boundary.

(Embedded bundle cache is already done; no work here.)

---

## 4) Clean Up Tests (avoid double coverage and false confidence)

### 4.1 Remove tests that validate the *old* behavior
Common dead tests after v2:
- tests asserting `ops.insufficient_context` for cases that are now canonicalized
- tests that build “weird shapes” only to prove old format-agnostic helpers work

Delete or rewrite them to validate v2 behavior:
- native manifest should deny for the *correct* reason
- `insufficient_context` only for genuinely missing required fields

### 4.2 Consolidate to three tiers of tests

**Tier A — Canonicalizer contract tests**
- shape extraction (Deployment/Pod/CronJob)
- casing normalization
- flat passthrough semantics

**Tier B — Policy rule unit tests**
- rules operate only on flat schema
- minimal dependencies on input shape

**Tier C — End-to-end regression tests**
- native manifest fixtures produce correct deny IDs
- never regress to false `ops.insufficient_context`

If a test does not fit any tier, it’s usually a candidate for deletion.

---

## 5) Update “Engine Logic v2” Documentation (single source of truth)

### 5.1 Update the canonical “how it works” doc
Create or update one document as the canonical reference:
- `docs/ENGINE_LOGIC_V2.md` (recommended name)

It should describe **only**:
- how input is formed (Go → `input.actions`)
- where normalization happens (canonicalize.rego → `defaults.actions`)
- what rules are allowed to read (defaults.actions only)
- how decisions are aggregated

It should explicitly state the invariants:
- no `input.actions` in rules/decision
- no K8s shape knowledge outside canonicalize.rego

### 5.2 Remove/merge outdated docs
If older documents describe the old behavior (Variant B helpers, reliance on flat input, etc.):
- mark them obsolete
- replace with links to `ENGINE_LOGIC_V2.md`
- delete if they confuse more than they help

A good rule:
> If a doc describes behavior that is no longer true after v2, it must be updated or removed in the same PR.

---

## 6) Final Refactor Pass (make v2 the clean baseline)

### 6.1 Flatten rules and simplify helpers
After canonicalization:
- rules should read like simple predicates on flat payload
- helpers should be small, reusable, and format-free

### 6.2 Tighten “insufficient_context”
Once false negatives are gone, tighten `insufficient_context` to:
- fire only when required fields are missing, not when nested

This is typically where you get the biggest UX win after canonicalization.

---

## 7) Recommended PR Structure (so cleanup actually happens)

### PR 1: Canonicalization boundary
- canonicalize.rego + defaults.actions
- switch all rules/decision to defaults.actions
- minimal tests to prove it works

### PR 2: Dead code + tests cleanup
- remove Variant-B helpers
- delete/update obsolete tests
- add CI guardrails

### PR 3: Engine v2 docs refresh
- add/update `docs/ENGINE_LOGIC_V2.md`
- remove/merge outdated docs

This separation prevents cleanup from being “postponed forever”.

---

## Implementation Tasks

### Step 1 — Dead code removal
- [ ] Remove all K8s shape/casing logic from defaults helpers and rules
- [ ] Delete unused format-detection helpers (Variant B artifacts)
- [ ] Verify `opa test` still passes

### Step 2 — Test cleanup and consolidation
- [ ] Delete obsolete tests that only validate pre-v2 behavior
- [ ] Ensure canonicalizer contract tests exist and are minimal
- [ ] Ensure native-fixture end-to-end rego tests exist (no false `insufficient_context`)

### Step 3 — CI guardrails
- [ ] Add grep/lint checks to prevent `input.actions` usage outside canonicalize.rego
- [ ] Add grep/lint checks to prevent K8s path walking outside canonicalize.rego
- [ ] Keep `opa test` in CI

### Step 4 — Documentation refresh
- [ ] Create/update `docs/ENGINE_LOGIC_V2.md` as the single source of truth
- [ ] Remove or mark obsolete any pre-v2 engine docs
- [ ] Ensure docs describe the actual current behavior

---
