You are acting as a Principal Software Architect performing a controlled architecture refinement.

Objective:
Refactor the existing architecture to replace the ad-hoc
thresholds/environments configuration model with a unified, OPA-native
data-driven "params" model.

This is a direct refactor (no migration, no backward compatibility needed).

Documents to update in-place:

1) AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
2) AI_ARCHITECTURE_GUARDRAILS_CLAUDE.md

Do NOT create new documents.
Return the fully updated versions of both files.

------------------------------------------------------------
HARD CONSTRAINTS (MUST REMAIN TRUE)
------------------------------------------------------------

- OPA Bundle is the sole policy artifact
- Single bundle per execution (no composition)
- Environment is opaque string label
- No environment enum
- No environment validation in Go
- No environment literals in Rego
- No numeric literals embedded in rule bodies
- Engine MUST NOT branch on semantic meaning of PolicyRef()
- Deterministic canonical JSON output required
- Deterministic tar.gz build required
- No SaaS
- No MCP
- No new architecture components

------------------------------------------------------------
PRIMARY CHANGE: UNIFIED PARAMS MODEL
------------------------------------------------------------

Replace any references to:

- thresholds.json
- environments.json
- data.evidra.data.thresholds
- data.evidra.data.environments

With a unified configuration model:

All tunable configuration MUST live under:

data.evidra.data.params

------------------------------------------------------------
PARAMS MODEL REQUIREMENTS
------------------------------------------------------------

1) Param Identity

Each tunable value is identified by a stable param_key string.
Param keys MUST NOT include:
- environment
- severity
- ordinal numbers
- version numbers

Examples (in words only, no code blocks):
- terraform.mass_delete.max_deletes
- k8s.namespaces.restricted
- argocd.destinations.allowed

Param keys describe configuration dimension only.

------------------------------------------------------------

2) Param Structure

Each param supports:

- by_env: map of environment_label -> value
- "default" entry inside by_env
- optional safety_fallback (documented hard safety constant)

Values may be:
- number
- boolean
- string
- array
- object
(standard JSON types only)

------------------------------------------------------------

3) Parameter Resolution Contract (NEW SECTION)

Add a normative section:

"Parameter Resolution Contract"

Define in strict MUST/MUST NOT language:

Resolution algorithm (described in words only):

1) Obtain environment_label from input context.
2) Lookup param.by_env[environment_label].
3) If absent, lookup param.by_env["default"].
4) If absent, use param.safety_fallback.
5) If still unresolved:
   - behavior must be explicitly defined by bundle author
   - either fail-open per rule OR explicit deny rule
   - NEVER engine-level default injection.

Clarify:
- Unknown environment MUST resolve deterministically.
- Engine does not inject defaults.
- Engine does not interpret environment semantics.

------------------------------------------------------------

4) Bundle Layout Update (System Design doc)

Update required internal bundle layout:

REMOVE:
- thresholds.json
- environments.json

REPLACE WITH:
- data/params.json (or equivalent file under data/)

Ensure no residual references remain.

------------------------------------------------------------

5) Acceptance Criteria Update

Update acceptance criteria to require:

- No references to thresholds or environments namespace.
- All tunables must exist under data.evidra.data.params.
- Rules must not contain numeric threshold literals.
- Unknown environment must produce deterministic output.
- Missing param behavior must be explicitly documented.

------------------------------------------------------------

6) Guardrails Update (Second Document)

Update AI_ARCHITECTURE_GUARDRAILS_CLAUDE.md:

Add release-blocking rules:

- Forbidden namespace usage:
  - data.evidra.data.thresholds
  - data.evidra.data.environments

- All tunables MUST exist under:
  - data.evidra.data.params

- Rule bodies must not contain:
  - hardcoded numeric thresholds
  - env string literals
  - references to thresholds or environments maps

- CI must detect:
  - numeric comparisons against constants inside rule logic
  - env string comparisons
  - direct map access bypassing param resolution contract

Prefer AST-based enforcement.
Regex allowed only as fallback.

------------------------------------------------------------

7) Remove Migration Language

If any section describes phased migration,
rewrite to reflect:

- Direct refactor.
- No backward compatibility required.
- Bundle-only model is authoritative from this version forward.

------------------------------------------------------------

OUTPUT FORMAT
------------------------------------------------------------

Return:

1) Updated .ai/AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
2) Updated .ai/AI_CLAUDE_ARCHITECTURE_GUARDRAILS_CLAUDE.md

Full documents, not diffs.

Use strict engineering tone.
No code blocks.
No pseudo-code.
No SaaS.
No MCP.
No architectural redesign.
Preserve determinism and artifact integrity guarantees.