You are writing a STRICT ARCHITECTURAL TECHNICAL SPECIFICATION.

Title:
"Environment-Agnostic, Data-Driven Policy Architecture (Strict Mode)"

Objective:

Introduce environment-aware configuration into Evidra using OPA bundles
with the following hard constraints:

HARD CONSTRAINTS (NON-NEGOTIABLE):

1) No hardcoded environment values anywhere in:
   - Go code
   - Rego files
   - CLI logic
2) No comparisons like:
   env == "prod"
   env == "dev"
3) No environment branching in Go.
4) No fixed enum of environments.
5) Environment must be treated as an opaque string label.
6) All behavior differences must be driven exclusively by policy data.
7) Policy resolution must remain deterministic.
8) No new DSL.
9) No registry.
10) No runtime config mutation.

Context:

Evidra uses:
- Rego policies
- Deterministic decision model
- Evidence recording
- OPA bundle layout

We are introducing:
- data.evidra configuration namespace
- environment-based thresholds and rule toggles
- policy bundle revision recording

Deliverable:

Produce a strict architectural specification.

No code.
No pseudo-code.
No marketing.
Engineering only.

------------------------------------------------------------
SECTION 1 — Architectural Invariants
------------------------------------------------------------

Define system-wide invariants:

- Environment is opaque.
- Engine does not interpret semantic meaning of environment.
- Policy bundle owns all environment semantics.
- CLI is transport only.
- Engine is execution only.
- Policy layer is decision only.
- Evidence layer is recording only.

Define non-violation rules:
- Any environment-dependent behavior must be resolved by data lookup.
- If a developer attempts to branch on environment literal, it violates architecture.

------------------------------------------------------------
SECTION 2 — Dependency Boundaries
------------------------------------------------------------

Explicitly define layer boundaries:

CLI Layer:
- Accepts environment string.
- Passes it through.
- No validation against enum.

Engine Layer:
- Does not interpret environment.
- Passes context to policy evaluator.
- Does not contain environment logic.

Policy Loader:
- Loads bundle.
- Loads data namespace.
- Does not transform environment value.

Rego Layer:
- May read input.context.environment.
- May read data.evidra.*
- Must not compare environment against literals.
- Must use lookup-based resolution only.

Evidence Layer:
- Records environment label.
- Records bundle revision.
- Does not derive meaning from environment.

Include a diagram description in text form.

------------------------------------------------------------
SECTION 3 — Namespace Isolation
------------------------------------------------------------

Define strict namespace rules:

- All configurable thresholds must live under:
  data.evidra.thresholds.<rule_id>.<env>

- All rule metadata must live under:
  data.evidra.rules.<rule_id>

- Defaults must live under:
  data.evidra.defaults

Prohibit:
- thresholds outside evidra namespace
- environment keys at top-level data
- rule toggles inside Rego constants

Define consequences of namespace violation.

------------------------------------------------------------
SECTION 4 — Deterministic Resolution Model
------------------------------------------------------------

Describe deterministic resolution rules in plain language:

Resolution order must be:

1) Input-provided environment label (if present)
2) Bundle default environment (if defined)
3) Fallback to literal "default" threshold (if present)
4) Hard safety fallback (documented constant)

Define:

- What happens if environment key is missing.
- What happens if threshold map is missing.
- What happens if rule metadata missing.
- What must never happen (panic, crash, nondeterminism).

No code allowed.

------------------------------------------------------------
SECTION 5 — Static Enforcement Strategy
------------------------------------------------------------

Define how architectural rules can be enforced:

- Code review rules.
- Lint rules for Rego (no string literal environment comparisons).
- Static grep rule example (conceptual).
- CI guardrails.
- Folder structure validation.

Describe enforcement strategy conceptually.

------------------------------------------------------------
SECTION 6 — Backward Compatibility
------------------------------------------------------------

Define:

- How existing rules migrate from hardcoded thresholds to data.
- How existing CLI invocations continue to work.
- Default behavior when no environment supplied.
- Migration path for existing bundles.

------------------------------------------------------------
SECTION 7 — Risk Model
------------------------------------------------------------

Identify risks:

- Developer introduces env branching in Go.
- Rule author hardcodes env in Rego.
- Bundle missing thresholds.
- User supplies unexpected environment.
- Data structure drift across bundles.

For each:
- Impact.
- Mitigation.
- Detection method.

------------------------------------------------------------
SECTION 8 — Acceptance Criteria
------------------------------------------------------------

Define measurable criteria:

- Adding new environment requires only editing data file.
- No code changes required.
- Rego files contain zero environment literals.
- Engine contains zero environment comparisons.
- Evidence records environment and bundle revision.
- System behavior remains deterministic under unknown environment.

------------------------------------------------------------
SECTION 9 — Explicit Non-Goals
------------------------------------------------------------

State clearly:

- No multi-environment inheritance.
- No environment validation registry.
- No runtime mutation of environment mapping.
- No remote dynamic environment resolution.
- No profile-env conflation.

------------------------------------------------------------
FORMAT RULES
------------------------------------------------------------

- Structured sections with headings.
- Clear bullet points.
- No code.
- No pseudo-code.
- No examples with literal environment values.
- Architecture-focused.
- Strict and formal tone.

End of specification.