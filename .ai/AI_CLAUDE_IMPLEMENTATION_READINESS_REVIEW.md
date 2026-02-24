# Implementation Readiness Review (Post-Hardening)

**Date:** 2026-02-24
**Reviewer:** Claude Opus 4.6 (Principal Architect role)
**Documents reviewed:**
- `.ai/AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md` (post-hardening)
- `.ai/AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md` (post-hardening)
**Predecessors:**
- `.ai/AI_CLAUDE_IMPLEMENTATION_READINESS_REVIEW.md` (initial review, 3 CRITs)
- Previous post-remediation review (this file, overwritten — composite 8.6)

---

## Section 1 — Overall Readiness Verdict

**Ready for implementation.**

All three original critical blockers remain resolved. The four non-critical items (REM-1 through REM-4) identified in the previous review have all been addressed in the micro-hardening pass. No new issues were introduced. A Go engineer can implement the bundle loader, evidence binding, parameter resolution helper, and CI guardrails from these documents without asking structural questions or making undocumented design decisions.

---

## Section 2 — Critical Issues (Must Fix Before Implementation)

### Previous CRIT-1 (OPA data file naming): RESOLVED (unchanged)

Bundle layout specifies `evidra/data/params/data.json` and `evidra/data/rule_hints/data.json`. Explicit statement that OPA only loads `data.json`/`data.yaml`. INV-15 and ANTI-10 enforce this. Data presence validation catches silent-ignore scenarios. Acceptance criteria 25, 26 cover both positive and negative cases.

### Previous CRIT-2 (Evidence binding interface): RESOLVED (unchanged)

`PolicySource` interface extended with `BundleRevision() string` and `ProfileName() string`. Method return table specifies behavior for both `LocalFileSource` (empty strings) and `BundleSource` (manifest values). INV-14 and ANTI-11 prohibit type assertions. Evidence model (§9) sources fields from explicit accessors. Acceptance criteria 27, 29 enforce the contract.

### Previous CRIT-3 (params JSON root structure): RESOLVED (unchanged)

"Data file content structure" subsection (§3) explicitly states: JSON root IS the params map, no wrapper key. JSON example demonstrates correct structure. Guardrails validation: "JSON root is a flat object (no wrapper key)." No remaining ambiguity.

### No new critical issues identified.

---

## Section 3 — Resolution Status of Previous REM Items

### REM-1 (PolicyRef content-integrity claim): RESOLVED

**Previous state:** The §9 authority table claimed PolicyRef provides "content integrity" for all sources, but `BundleSource.PolicyRef()` returns the manifest revision — not a content hash. The "Content integrity" row was misleading for bundle mode.

**Current state:** The authority table (§9) now correctly states: "Content integrity: 'were the bytes tampered with?' — Authoritative field: Release artifact checksum + build/release pipeline." The `PolicyRef` role column explicitly distinguishes the two modes: for `LocalFileSource`, PolicyRef is a content hash and MAY serve as an integrity indicator; for `BundleSource`, PolicyRef returns the manifest revision and content integrity is provided by the release artifact checksum and the build/release pipeline. The §7 semantic distinction paragraph now includes the statement that `PolicyRef` "MUST NOT be used for integrity or provenance decisions when `BundleRevision` is present." Guardrails INV-9 mirrors this language.

**Verdict:** No remaining inconsistency. The content-integrity use case is correctly attributed to the release pipeline for bundle mode, and to PolicyRef for LocalFileSource mode. Both §7 and §9 are internally consistent and cross-consistent with INV-9.

### REM-2 (Parameter Resolution Contract helper unnamed): RESOLVED

**Previous state:** ANTI-9 mandated "a shared Rego helper" but did not specify its name, package, or location. A developer had to invent these.

**Current state:** System Design §4 now specifies the canonical helper: name `resolve_param`, location `evidra/policy/defaults.rego`, package `evidra.policy`. Responsibilities are defined in words: env-specific lookup, `by_env["default"]` fallback, `safety_fallback` fallback, unresolved behavior. All Rego rules MUST call `resolve_param` rather than implementing the chain inline. Guardrails ANTI-9 references the same name and location. The Rego lint table and drift detection table both reference `resolve_param` by name.

**Verdict:** Fully specified. A developer can implement `resolve_param` from the §4 contract definition without ambiguity about its name, location, package, or responsibilities. All cross-references are consistent.

### REM-3 (INV-2 enforcement scope open-ended): RESOLVED

**Previous state:** INV-2 prohibited "any other infrastructure-specific identifiers" — an unbounded category. The CI regex blocklist was finite. The gap between normative text and enforcement was undocumented.

**Current state:** INV-2 now includes an explicit "Enforcement scope" paragraph: "CI enforces only the explicit, documented subset of infrastructure-specific patterns listed in the CI lint rules (§3). Any additional infrastructure-specific identifier patterns beyond the CI blocklist are subject to code review until explicitly added to the CI pattern set." The Rego lint table intro mirrors this: "This pattern set is the enforced subset of INV-2."

**Verdict:** The normative text and enforcement mechanism are now aligned. The invariant remains aspirationally broad (covering all infrastructure-specific identifiers), while the enforcement scope is honestly documented as the CI-checked subset plus code review. No developer confusion about what is automated vs manual.

### REM-4 (Determinism test N is a range): RESOLVED

**Previous state:** "N >= 10" allowed variation across CI runs.

**Current state:** Both references now specify `N = 50`: §8 verification ("N times (N = 50)") and acceptance criterion 13 ("N = 50 evaluations"). No other N references exist in either document.

**Verdict:** Fixed value. No variation across CI runs. Both references are consistent.

---

## Section 4 — Ambiguity Map

| Location | Ambiguity | Severity | Status |
|---|---|---|---|
| System Design §7/§9: `PolicyRef` content-integrity role | Previously inconsistent for BundleSource | — | **RESOLVED.** Authority table now correctly attributes content integrity to release artifact checksum for bundle mode. |
| Guardrails ANTI-9: "shared Rego helper" | Previously unnamed | — | **RESOLVED.** `resolve_param` in `evidra/policy/defaults.rego`, package `evidra.policy`. |
| Guardrails INV-2: "infrastructure-specific identifiers" scope | Previously open-ended | — | **RESOLVED.** CI blocklist is the enforced subset; code review covers the remainder. |
| System Design §8: determinism test N | Previously a range | — | **RESOLVED.** N = 50, fixed. |

### Previously resolved ambiguities (unchanged from last review):
- `--profile` flag: defined (§1, hard failure on mismatch) ✓
- `PolicySource` vs `BundleArtifact` metadata access: resolved (explicit interface methods) ✓
- `params.json` filename: resolved (subdirectory `data.json`) ✓
- Top-level key wrapper: resolved (no wrapper, documented with example) ✓
- `LoadData` merge ownership: resolved (OPA's native merge, no custom logic) ✓
- Resolution step 5 "explicitly defined": resolved (`unresolved_behavior` field) ✓
- `PolicyRef` population for bundles: defined (method return table in §7) ✓
- `BundleRevision` empty permissibility: defined (INV-5 + §9) ✓
- Acceptance criterion 22 scope: tightened (no tunables outside params) ✓

### Remaining ambiguities: NONE

No open ambiguity items remain in either document. Every structural question that a Go or Rego engineer would need to answer during implementation is addressed with an explicit, unambiguous statement.

---

## Section 5 — Determinism Audit

### Byte-identical replay: GUARANTEED (unchanged)

The determinism chain is complete:
- **Input identity:** `InputHash` — SHA-256 of canonical JSON of input document. Canonical JSON fully specified (§8).
- **Policy identity:** `BundleRevision` — immutable manifest revision. 1:1:1 mapping to artifact (§8).
- **Environment identity:** `EnvironmentLabel` — recorded verbatim from caller input (§9).
- **Output ordering:** Lexicographic sort on `Hits`, `Hints`, `Reasons` — unconditional (§8, INV-8).
- **Serialization:** Canonical JSON — key ordering, whitespace, unicode, null handling all specified (§8).
- **No implicit state:** Explicitly prohibited — no clock, no random, no network, no env vars during evaluation (§8).
- **Test count:** Fixed at N = 50 — no variation across CI runs (§8).

Given `BundleRevision` + `InputHash` + `EnvironmentLabel`, any party can retrieve the exact bundle artifact, reconstruct the exact input, and produce a byte-identical decision.

### Artifact identity chain: COMPLETE (unchanged)

```
Git tag
  → manifest revision (injected at build time)
    → artifact filename (must match)
      → archive SHA-256 checksum
        → evidence record (bundle_revision field)
```

Each link is defined with exact-match requirements. The 1:1:1 mapping (tag:revision:artifact) is unconditional with no exception mechanism (INV-6).

### Content integrity: CORRECTLY ATTRIBUTED

Content integrity in bundle mode is provided by the release artifact checksum and the build/release pipeline (§9). `PolicyRef` is informational only in this mode. For `LocalFileSource`, `PolicyRef` (content hash) MAY serve as an integrity indicator. This distinction is now explicitly documented in both §7 and §9, and enforced by the MUST NOT language in the semantic distinction paragraph.

### Implicit runtime state: NONE DETECTED (unchanged)

All prohibited state sources are enumerated and enforced:
- System clock: prohibited during OPA evaluation
- Random values: prohibited
- Network lookups: prohibited
- Environment variable inspection during evaluation: prohibited
- Map iteration order: sorted before output (INV-8)
- Data merge: delegated to OPA's deterministic bundle loader (§7)

---

## Section 6 — Cross-Document Consistency Audit

This section verifies that the two documents do not contradict each other on any point.

| Concept | System Design location | Guardrails location | Consistent? |
|---|---|---|---|
| `resolve_param` helper | §4: name, location, package, responsibilities | ANTI-9: same name and location; lint table: `resolve_param`; drift table: `resolve_param` | ✓ |
| PolicyRef integrity role in bundle mode | §7: "MUST NOT use for integrity or provenance when BundleRevision present"; §9: release artifact checksum is authoritative | INV-9: "MUST NOT be used for integrity or provenance decisions when BundleRevision is present"; "content integrity in bundle mode is provided by the release artifact checksum" | ✓ |
| INV-2 enforcement scope | §5: environment literals prohibited (normative) | INV-2: enforcement scope paragraph; lint table intro: "enforced subset" | ✓ |
| Determinism test N | §8: "N = 50"; AC-13: "N = 50" | No direct reference (guardrails does not specify N) | ✓ (no conflict) |
| PolicySource interface | §7: 5 methods, return table | INV-14: no type assertions; INV-5: BundleRevision() for evidence | ✓ |
| Data file naming | §3: `data.json` in subdirectories; INV-15 referenced | INV-15: same; ANTI-10: same; lint/namespace tables: same | ✓ |
| Forbidden namespaces | §3: thresholds, environments | INV-12: same; ANTI-8: same | ✓ |
| Param key encoding | §4: no environment, severity, ordinal, version | INV-13: same | ✓ |
| `--profile` semantics | §1: selection + validation, hard failure on mismatch | Not directly in guardrails (engine behavior, not guardrail) | ✓ (no conflict) |
| Bundle author obligations | §4: unresolved_behavior; §5: catch-all deny rule | INV-10: engine must not implement; ANTI-5: no engine defaults | ✓ |

No cross-document inconsistencies detected.

---

## Section 7 — Final Confidence Score

| Dimension | Score (1-10) | Change from previous | Justification |
|---|---|---|---|
| **Architectural clarity** | 10 | +1 | All four REM items resolved. PolicyRef integrity role correctly attributed per source type. `resolve_param` helper fully specified. No remaining ambiguities in ambiguity map. |
| **Enforceability** | 9 | +1 | INV-2 enforcement scope explicitly documented (CI subset + code review). Lint table and drift table reference `resolve_param` by name. ANTI-9 `by_env` check remains advisory (appropriate — AST-based promotion path is documented). |
| **Determinism rigor** | 10 | +1 | Determinism test N fixed at 50. Content integrity correctly attributed to release pipeline for bundle mode. All replay fields fully specified with no gaps. |
| **Drift resistance** | 9 | — | 18 drift signals (unchanged). `resolve_param` named in drift table enables precise detection of direct `by_env` access. INV-2 enforcement scope clarification prevents false expectations about automated vs manual detection. |
| **Implementation readiness** | 10 | +2 | Zero open ambiguities. Zero undocumented decisions. `resolve_param` name, location, and package are specified. Determinism test count is fixed. Content integrity attribution is unambiguous. A Go or Rego engineer can implement from these documents without asking any clarifying questions. |

**Composite: 9.6 / 10** (previous: 8.6)

The documents are fully implementation-ready. No structural, naming, or consistency issues remain. The 0.4-point gap to 10.0 reflects two inherent characteristics that are by-design, not deficiencies:
- ANTI-9 `by_env` direct-access check is advisory (appropriate until AST-based enforcement is available)
- INV-2 enforcement beyond the CI blocklist relies on code review (the enforcement scope paragraph explicitly documents this)

Both are deliberate architectural choices with documented rationale, not gaps to be closed.
