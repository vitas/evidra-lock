You are acting as a Principal Software Architect performing a targeted remediation pass based on an Implementation Readiness Review.

Goal:
Apply ONLY the structural fixes required to make the architecture documents implementation-ready, exactly as recommended in the readiness review, and only if you (the architect) agree the fixes are correct.

Input documents to update in-place:
1) AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
2) AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md

Reference review document:
- AI_CLAUDE_IMPLEMENTATION_READINESS_REVIEW.md

Output:
Return the fully updated versions of both documents (full text, not diffs).

Constraints (must remain true):
- OPA Bundle is the sole policy artifact
- Single bundle per execution (no multi-bundle, no layering)
- No SaaS
- No MCP
- Environment is opaque string label (no enums, no Go validation)
- No environment literals in Rego
- Deterministic canonical JSON output required
- Engine MUST NOT branch on semantic meaning of PolicyRef()
- Strict engineering tone
- No code, no pseudo-code

--------------------------------------------------------------------
STEP 0 — Architect Agreement Gate
--------------------------------------------------------------------
Before applying changes, explicitly state:
- "AGREE" or "DISAGREE" with each proposed fix below.
If you disagree, explain briefly and propose a minimally invasive alternative that still resolves the blocker.
Then proceed with the agreed fixes.

--------------------------------------------------------------------
FIX SET (from readiness review) — MUST ADDRESS
--------------------------------------------------------------------

CRIT-1 / CRIT-3: OPA bundle data loading compatibility

Problem:
OPA will not load arbitrary data files named params.json / rule_hints.json unless they are named data.json/data.yaml or are otherwise in a recognized OPA data document path. The current design risks silent data omission, making lookups undefined.

Required fix (preferred approach):
Update the bundle layout and doc text to use directory-based OPA-native data loading:

- evidra/data/params/data.json      -> maps to data.evidra.data.params
- evidra/data/rule_hints/data.json  -> maps to data.evidra.data.rule_hints

Additionally:
- Remove any requirement that the file content must have a top-level "params" key.
- The JSON root in evidra/data/params/data.json MUST be the params map itself (param_key -> param object), so that the namespace is exactly data.evidra.data.params[param_key]...

Make the change in:
- Bundle layout section
- Data namespace mapping table
- Any examples or acceptance criteria referencing params.json / rule_hints.json
- Guardrails: update path/namespace references accordingly

Also update validation requirements:
- Bundle loader MUST validate that these data documents are present and loadable by OPA (fail fast).
- Bundle loader MUST fail if params data is absent or empty when required by policy pack.

--------------------------------------------------------------------

CRIT-2: Evidence binding requires BundleRevision and ProfileName access

Problem:
The engine cannot reliably populate evidence fields (bundle_revision, profile_name) if it only receives PolicySource and PolicyRef() is opaque and must not be semantically parsed.

Required fix:
Update architecture to extend the PolicySource contract (or equivalent boundary) with explicit metadata accessors.

Define (normatively):
- PolicySource.BundleRevision() string
- PolicySource.ProfileName() string
- For LocalFileSource these MUST return empty string.
- For BundleSource these MUST return manifest revision and manifest profile_name.

Additionally:
- Engine MUST NOT infer revision/profile from PolicyRef().
- Engine MUST NOT type-assert concrete sources for metadata.
- Evidence MUST be populated from these explicit accessors.

Update:
- System Design: interfaces/contracts, evidence model, determinism invariants, acceptance criteria
- Guardrails: add release-blocking rule forbidding semantic branching on PolicyRef and forbidding type assertions for metadata

--------------------------------------------------------------------
MINIMAL OPTIONAL IMPROVEMENTS (only if architect agrees)
--------------------------------------------------------------------

IMP-1: Define --profile semantics precisely
- --profile selects a bundle directory under policies/bundles/<profile>
- Manifest metadata.profile_name is authoritative identity.
- Clarify whether mismatch between CLI profile name and manifest profile_name is:
  (a) hard failure, or (b) allowed but recorded (choose one; prefer hard failure for determinism).

IMP-2: Clarify data merge semantics
- State that data tree is the result of OPA bundle loader’s standard merge behavior.
- Prohibit custom merge logic in Evidra beyond OPA’s native behavior.

Do not add any other improvements beyond these.

--------------------------------------------------------------------
OUTPUT FORMAT
--------------------------------------------------------------------

1) Provide updated AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md (full document)
2) Provide updated AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md (full document)

Before the documents, include a short "Agreement Summary" section listing:
- CRIT-1/3: AGREE/DISAGREE + rationale
- CRIT-2: AGREE/DISAGREE + rationale
- IMP-1: AGREE/DISAGREE + rationale
- IMP-2: AGREE/DISAGREE + rationale

No code.
No pseudo-code.
No new components.
No new features.
Strict, implementation-ready language (MUST/MUST NOT).