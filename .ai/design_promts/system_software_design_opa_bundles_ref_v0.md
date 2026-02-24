You are acting as a Principal Software Architect.

Task:
Design the SYSTEM and SOFTWARE ARCHITECTURE for introducing
OPA Bundles as the PRIMARY and STANDARD mechanism
for policy definition, packaging, and execution in Evidra.

OPA Bundle must be treated as the authoritative policy artifact format.

This architecture must:

- Use official OPA bundle structure strictly
- Be fully data-driven (no hardcoded thresholds)
- Treat environment as opaque string label
- Support single-bundle per execution (MVP)
- Preserve strict determinism
- Avoid multi-bundle composition
- Avoid custom DSL
- Avoid environment enums
- Avoid environment branching in Go
- Avoid SaaS considerations entirely

NO CODE.
NO PSEUDO-CODE.
STRICT ENGINEERING ARCHITECTURE ONLY.

------------------------------------------------------------
REQUIRED DELIVERABLES
------------------------------------------------------------

You must produce the following files:

1) ai/AI_SYSTEM_DESIGN_OPA_BUNDLES.md
   - Main system & software architecture document
   - Structured sections (see below)
   - Text-based component diagrams
   - Clear dependency boundaries
   - Deterministic resolution description

2) ai/AI_BUNDLE_BUILD_AND_RELEASE_STRATEGY.md
   - Bundle build model
   - Versioning model
   - GitHub Release artifact strategy
   - Integrity & reproducibility model

3) ai/AI_ARCHITECTURE_GUARDRAILS.md
   - Explicit architectural invariants
   - Anti-patterns (what must never be introduced)
   - CI/Review enforcement guidelines
   - Drift detection strategy

Each document must be production-quality architecture documentation.

------------------------------------------------------------
DOCUMENT 1 STRUCTURE:
AI_SYSTEM_DESIGN_OPA_BUNDLES.md
------------------------------------------------------------

Must contain:

1. Architectural Overview
   - Current state
   - Target state
   - Text diagram:
     CLI → Engine → Bundle Loader → OPA Evaluation → Evidence

2. Strategic Decision: OPA Bundle as Sole Policy Artifact
   - Why no custom format
   - Why bundle revision is authoritative

3. Bundle-Based Policy Architecture
   - Required directory structure
   - Manifest requirements
   - Data namespace isolation

4. Data-Driven Environment Model
   - Environment as opaque string
   - No fixed env enum
   - No env literals in Rego
   - No env branching in Go
   - Deterministic resolution model (in words only)

5. Single-Bundle Execution Model
   - Exactly one bundle active
   - No composition
   - No merge strategy

6. Software Layering & Dependency Boundaries
   - CLI
   - Engine
   - Bundle Loader
   - OPA Evaluation
   - Evidence
   - Allowed dependency direction
   - Forbidden dependencies

7. Determinism Model
   - Same input + same revision + same env → same decision
   - Immutability requirement for BundleArtifact

8. Evidence Model Extension
   - bundle_revision
   - profile_name
   - environment_label
   - input_hash
   - Why binding revision is mandatory

9. Migration Strategy
   - Moving inline rules to bundle layout
   - Moving hardcoded thresholds to data namespace

10. Acceptance Criteria
   - Measurable architectural checks

------------------------------------------------------------
DOCUMENT 2 STRUCTURE:
AI_BUNDLE_BUILD_AND_RELEASE_STRATEGY.md
------------------------------------------------------------

Must contain:

1. Bundle Source Layout
   - Repository structure
   - Versioning conventions

2. Bundle Build Process
   - Validation steps
   - Manifest revision requirements
   - Deterministic build expectations

3. Versioning Strategy
   - Manifest revision format
   - Mapping to Git tags

4. GitHub Release Model
   - Artifact naming conventions
   - tar.gz bundle packaging
   - Optional checksum
   - Release notes policy

5. Integrity & Reproducibility
   - Why revision must match artifact
   - Why builds must be deterministic
   - Validation before publishing

6. Non-Goals
   - No remote registry
   - No dynamic runtime downloads

------------------------------------------------------------
DOCUMENT 3 STRUCTURE:
AI_ARCHITECTURE_GUARDRAILS.md
------------------------------------------------------------

Must contain:

1. Architectural Invariants
   - No environment branching in Go
   - No env literals in Rego
   - All thresholds in data namespace
   - Single-bundle execution only

2. Prohibited Patterns
   - Hardcoded thresholds
   - Inline environment checks
   - Silent fallback bundles
   - Multi-bundle creep

3. CI Enforcement Strategy
   - Linting rules
   - Review checklist
   - Manifest validation
   - Namespace validation

4. Drift Detection Model
   - How to detect accidental architectural violations

------------------------------------------------------------
GLOBAL CONSTRAINTS
------------------------------------------------------------

- No code
- No pseudo-code
- No SaaS discussion
- No MCP discussion
- No marketing language
- Strict engineering tone
- OPA Bundle as authoritative standard
- Emphasis on determinism
- Emphasis on immutability
- Emphasis on architectural integrity

Output must be structured, precise, and implementation-ready.