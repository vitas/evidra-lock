PHASE 4 PROMPT — Repo-Wide Consistency & Drift Sweep (CHECKS + MINIMAL CLEANUPS)

You are in PHASE 4 of a controlled refactor.

Primary goal:
Prove the repository is internally consistent and drift-free after implementation and doc updates.

Rules:
- This phase is primarily verification.
- Allowed changes are MINIMAL CLEANUPS ONLY:
  - Remove obsolete/unreferenced files left from refactor
  - Fix broken doc links or path typos in examples/tests
  - Tighten CI grep patterns ONLY to match the guardrails intent (no weakening)
- Do NOT introduce new features.
- Do NOT redesign architecture.
- No SaaS, no MCP, no multi-bundle.

References (must match):
- AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
- AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md

Checklist to execute and report:

A) Repo-wide terminology sweep
- Confirm zero remaining references to legacy concepts/paths:
  - "thresholds"
  - "environments"
  - old pack paths that no longer exist
- Confirm no references to non-OPA-loadable data file naming (e.g., params.json outside data.json conventions)

B) Bundle structure validation
For each bundle under policies/bundles/* verify:
- .manifest exists at bundle root
- roots == ["evidra"] (MVP strict)
- metadata.profile_name exists and matches bundle directory name (per spec)
- OPA-loadable data docs exist under evidra/data/**/data.json
- expected namespaces exist:
  - data.evidra.data.params
  - data.evidra.data.rule_hints (if required)
Report any missing elements as FAIL.

C) Guardrails verification (static)
- Confirm no env string literal comparisons in Rego
- Confirm forbidden namespaces do not exist anywhere:
  - data.evidra.data.thresholds
  - data.evidra.data.environments
- Confirm Engine does not branch on PolicyRef semantics:
  - no prefix checks, regex checks, or format inference on PolicyRef()
- Confirm Engine does not type-assert PolicySource concrete types for metadata

D) Determinism verification (runtime)
- Run determinism test N=50 (as specified)
- Confirm byte-identical canonical JSON output across runs
- Confirm stable sorting of violations

E) Documentation verification
- All bundle tree examples in docs match actual repo layout
- All command examples use current flags/paths
- No stale references remain

Output format:
1) A pass/fail table for A–E
2) A list of any issues found with exact file paths and snippets (short)
3) A list of minimal cleanups performed (if any)
4) Final statement:
   - Only say “Repo is consistent” if all checks pass

Do NOT:
- Make broad refactors
- Change core logic
- Weaken guardrails

Deliverable:
Return the verification report plus any minimal cleanup summary.