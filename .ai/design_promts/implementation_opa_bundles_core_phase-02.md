You are a senior Go engineer implementing the architecture defined in these documents:

- .ai/AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
- .ai/AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md

Goal:
Implement and refactor the codebase to match the architecture exactly, including all invariants and guardrails.

Hard constraints (must hold in the implementation):
- OPA Bundle is the sole policy artifact format.
- Exactly ONE bundle active per execution (reject multi-bundle inputs early).
- Environment is an opaque string label (no enums, no validation in Go).
- No environment literals in Rego.
- Engine MUST NOT branch on semantic meaning of PolicyRef() (opaque string; no parsing/prefix checks/regex/length inference).
- No type assertions in Engine to obtain bundle metadata; use explicit interface accessors.
- Deterministic outputs:
  - stable sorting of violations
  - canonical JSON serialization
  - determinism test with N=50 repeated runs
- OPA data docs must be loadable by OPA:
  - only data.json/data.yaml conventions
  - directory-based mapping into data namespace
- No SaaS considerations.
- No MCP considerations.
- No multi-bundle composition, merging, or layering.

Do NOT implement new product features.
Do NOT redesign the architecture.
Follow the documents strictly.

Deliverables:
1) Code changes implementing:
   - Bundle loading
   - Manifest validation
   - Policy evaluation wiring
   - Evidence metadata binding
   - Deterministic output ordering + canonical JSON
   - CLI profile selection behavior
2) Repo changes implementing guardrails:
   - CI checks (regex now, AST later if specified)
   - release-blocking checks for forbidden patterns/namespaces
3) Update internal policy pack layout (bundle content) to match the documented structure.

No need for backward compatibility/migration: refactor directly.

--------------------------------------------------------------------
PHASE 0 — Read & Extract Requirements (must produce)
--------------------------------------------------------------------
Create a short “Implementation Notes” section in your work log (not as a doc file) that lists:
- All MUST/MUST NOT requirements you will enforce
- All acceptance criteria you will test
- The exact bundle directory layout you will implement
This is for your own planning; do not add new docs.

--------------------------------------------------------------------
PHASE 1 — Bundle Artifact Layout (policy repository structure)
--------------------------------------------------------------------
Implement the documented OPA-compliant bundle internal structure:

At minimum ensure:
- .manifest exists and is at bundle root
- manifest includes required fields:
  - revision (non-empty)
  - roots (MVP: exactly ["evidra"])
  - metadata.profile_name (authoritative)
- Data documents are OPA-loadable using directory-based data.json:
  - evidra/data/params/data.json        -> data.evidra.data.params
  - evidra/data/rule_hints/data.json    -> data.evidra.data.rule_hints
  (and any other documented data namespaces)
- Rego modules live under evidra/policy/...

Ensure your bundle build artifacts follow the design docs (tar.gz with deterministic settings is handled by build pipeline if in scope; do not introduce new build systems beyond what docs require).

--------------------------------------------------------------------
PHASE 2 — PolicySource contract changes (metadata access without branching)
--------------------------------------------------------------------
Refactor code so the Engine can access bundle metadata without:
- parsing PolicyRef()
- type asserting concrete sources

Implement explicit accessors on the policy source boundary:
- BundleRevision() string  (empty for LocalFileSource)
- ProfileName() string     (empty for LocalFileSource)

Ensure:
- BundleSource returns manifest revision and manifest metadata.profile_name
- LocalFileSource returns empty strings
- Engine reads these fields directly (no special casing based on string format)

Add/adjust any interfaces and constructors accordingly.

--------------------------------------------------------------------
PHASE 3 — Bundle Loader (OPA-native loading + strict validation)
--------------------------------------------------------------------
Implement bundle loader responsibilities:

- Validate bundle directory structure exists
- Validate .manifest presence and required fields
- Validate roots == ["evidra"] (MVP strict)
- Validate metadata.profile_name is present and non-empty
- Load Rego modules
- Load OPA data documents using OPA’s standard bundle/data loading semantics
- Fail fast if expected OPA data docs are missing or not loaded:
  - params namespace must exist and be loadable
  - rule_hints namespace must exist per design doc rules (match doc strictness)
- Do NOT mutate data
- Do NOT implement custom merge logic (use OPA loader behavior only)
- Produce an immutable BundleArtifact (conceptual) that the Engine can evaluate

--------------------------------------------------------------------
PHASE 4 — CLI: --profile semantics and selection
--------------------------------------------------------------------
Implement CLI behavior as defined:

- --profile selects bundle directory under policies/bundles/<profile>
- Compare CLI profile name with manifest metadata.profile_name:
  - follow the spec’s rule (likely hard failure on mismatch)
- Provide a clear error before evaluation if mismatch occurs
- Environment label input:
  - accept as opaque string
  - pass through to input context
  - do not validate against any enum

Reject invalid inputs early:
- multiple bundle paths
- directories containing multiple bundles
- ambiguous selection

--------------------------------------------------------------------
PHASE 5 — Policy Evaluation + Params Resolution Helper (Rego side)
--------------------------------------------------------------------
Implement the policy pack conventions required by the docs:

- Create/ensure the canonical helper exists:
  - helper name: resolve_param
  - file: evidra/policy/defaults.rego
  - package: evidra.policy (or the canonical package in docs)

The helper must implement (per docs) the param resolution logic:
- read environment label from input context
- resolve values from data.evidra.data.params
- apply the fallback chain as described in the design docs
- implement unresolved behavior exactly as specified (no invention)

Ensure:
- No env string literal comparisons in Rego
- No tunable numeric literals in rule bodies (tunables must come from params)

--------------------------------------------------------------------
PHASE 6 — Deterministic Output Contract
--------------------------------------------------------------------
Implement deterministic output requirements:

- Define a stable sorting strategy for:
  - violations array (primary key: rule_id; then stable secondary keys as spec defines)
  - any nested arrays that may affect output determinism
- Ensure no reliance on map iteration order

Canonical JSON serialization:
- Use canonical JSON encoding rules defined in the design docs
- Ensure stable key ordering
- No insignificant whitespace differences
- UTF-8 byte identity
- No custom marshalers that violate canonical output constraints

Determinism test harness:
- Run the same evaluation N=50 times
- Assert byte-identical output across runs
- This test must be release-blocking in CI

--------------------------------------------------------------------
PHASE 7 — Evidence Model Binding
--------------------------------------------------------------------
Update evidence record writing to include (as required by docs):
- bundle_revision (from PolicySource.BundleRevision())
- profile_name (from PolicySource.ProfileName())
- environment_label (from input context)
- input_hash (from canonical input normalization)
- decision + violations

Rules:
- Engine must not infer these from PolicyRef()
- Evidence must be reproducible and replayable under the same bundle revision + same input

--------------------------------------------------------------------
PHASE 8 — Guardrails & CI enforcement (release-blocking)
--------------------------------------------------------------------
Implement CI checks described in .ai/AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md:

Release-blocking checks must include (at minimum):
- No env string literal comparisons in Rego
- No forbidden namespaces:
  - data.evidra.data.thresholds
  - data.evidra.data.environments
- Data documents must be named data.json/data.yaml under correct dirs
- No PolicyRef() semantic branching patterns in Go
- No type assertions in Engine for policy metadata
- Single-bundle enforcement tests
- Determinism test must pass (N=50)

If guardrails specify AST-preferred checks but allow regex fallback:
- implement regex fallback now (narrow, minimal false positives)
- leave clear TODOs for AST implementation without changing scope

--------------------------------------------------------------------
FINAL ACCEPTANCE CHECKLIST (must pass before “done”)
--------------------------------------------------------------------
- Bundle loads via OPA-native loader with data.json-based mapping
- Missing manifest fields cause hard failure
- roots == ["evidra"] enforced (MVP)
- CLI profile mismatch with manifest profile_name fails early
- Engine never branches on PolicyRef semantics
- Engine obtains revision/profile only via explicit accessors
- Evidence includes required metadata fields
- Determinism: byte-identical output across N=50 runs
- Guardrails: CI blocks forbidden patterns and namespaces
- No multi-bundle support exists; multiple inputs are rejected

Output requirements for your response/work:
- Provide a structured implementation plan as a checklist (phases above)
- Provide a list of files/packages likely to change
- Provide a test plan mapping acceptance criteria -> test cases
- Do NOT write full code in this response unless explicitly requested
- If you propose any deviation, you must justify it by quoting the relevant invariant it preserves