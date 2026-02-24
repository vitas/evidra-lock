PHASE 3 PROMPT — Documentation & Public Surface Update (DOCS ONLY)

You are in PHASE 3 of a controlled refactor.

Rules:
- DOCS CHANGES ONLY. Do NOT modify Go/Rego code or CI logic in this phase.
- Do NOT redesign architecture.
- Do NOT introduce new features.
- No SaaS, no MCP, no multi-bundle.
- Everything must match:
  - AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
  - AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md

Objective:
Update ALL public and internal documentation so it reflects the new OPA bundle architecture and contains zero legacy guidance.

Scope of updates (must cover all that exist in the repo):
1) Root README.md
   - Describe OPA bundle layout with an accurate tree
   - Explain --profile semantics
   - Explain environment as opaque label
   - Explain deterministic output guarantees (canonical JSON, repeated-run determinism)
   - Explain evidence fields (bundle_revision, profile_name, environment_label, input_hash)
   - Update examples of running the tool (paths/flags)

2) docs/ directory (if present)
   - Update all bundle structure examples to OPA-native data.json layout
   - Remove/replace all references to legacy namespaces/terms:
     - thresholds
     - environments
   - Update “how to author policies” guidance:
     - params namespace
     - rule ID naming standard (if documented)
     - no env literals in Rego

3) Examples and sample bundles
   - Ensure sample bundle(s) are valid OPA bundles
   - Ensure data files are named data.json and placed in correct dirs
   - Ensure example manifests contain required fields and match profile naming rules

4) CI / guardrails documentation (if present)
   - Explain release-blocking checks at a high level
   - Ensure wording matches guardrails doc (no contradictions)

5) Links and references
   - Fix broken links caused by renames/moves
   - Ensure no stale file paths remain in docs

Output requirements:
- Provide a list of every documentation file you edited
- For each file: a short summary of what changed
- Provide a “Doc Consistency Checklist” confirming:
  - No remaining references to thresholds/environments
  - All bundle tree examples match actual repo structure
  - All command examples use current flags/paths
  - No multi-bundle guidance exists
  - No SaaS/MCP mentions exist

Do NOT:
- Edit code or CI configs
- Add new docs beyond what’s necessary (only update existing docs unless a missing doc is required by the architecture docs)
- Leave ambiguous instructions

Deliverable:
Return the updated documentation content (or a patch plan if you cannot directly edit files in this environment), plus the edited-files list and checklist.