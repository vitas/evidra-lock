# AI System Design: OPA Bundles as Primary Policy Standard

## 1. Architectural Overview

### Terminology
- **Bundle artifact**: one OPA bundle archive used for one evaluation.
- **Profile**: policy family name represented by a bundle artifact.
- **Policy roots**: namespace roots declared in `.manifest.roots`.

### Current State
- Policy is authored as Rego + JSON data files and loaded directly for evaluation.
- Policy logic and policy data are already deterministic, but artifact packaging is not yet standardized around one authoritative unit.
- Evidence records decisions, but policy artifact identity must become first-class and immutable.

### Target State
- OPA Bundle becomes the sole policy artifact format for definition, packaging, transport, and execution.
- Every evaluation uses exactly one bundle artifact with a single authoritative manifest revision.
- Evidence binds every decision to the exact bundle revision and input hash.

### Component Diagram
```text
[CLI]
  -> [Engine]
      -> [Bundle Loader]
          -> [OPA Evaluation]
              -> [Evidence]
```

## 2. Strategic Decision: OPA Bundle as Sole Policy Artifact

- OPA Bundle is an established standard; no custom policy packaging format is introduced.
- Bundle `.manifest.revision` is the authoritative policy identity for runtime and audit.
- `policy_ref` is secondary and informational only. It must not override, replace, or reinterpret `bundle_revision`.
- Policy behavior is tracked by `bundle_revision`, not by repository path, branch, deployment timestamp, or mutable labels.
- Manifest revision, release artifact identity, and release metadata must match exactly.
- Re-tagging different bundle content under the same manifest revision is forbidden.
- Bundle artifact immutability is absolute after release publication.

## 3. Bundle-Based Policy Architecture

### Required Directory Structure (inside bundle)
- `.manifest` at bundle root.
- Rego modules under declared policy roots.
- Data documents (`*.json`) under the same policy roots.
- No policy inputs from undeclared paths.

### Manifest Requirements
- `revision` is mandatory and immutable for a released artifact.
- `roots` is mandatory and defines policy roots for all loaded policy/data namespaces.
- Manifest must be validated before evaluation starts.

### Data Namespace Isolation
- All thresholds, limits, and policy metadata must live under namespaced data in policy roots.
- No runtime-only side tables for policy thresholds.
- No mixed ownership between bundle data and Go constants.

## 4. Data-Driven Environment Model

- Environment is an opaque string label passed as input context.
- No fixed environment enum in Go.
- No environment literals embedded in Rego rules.
- No Go-side branching based on environment values.
- Engine never injects a default environment value.
- Go runtime must not validate environment semantics (allowed values, categories, or naming rules).

### Deterministic Resolution Model
- Engine forwards the environment label unchanged.
- Policy resolves environment-specific configuration by data lookup using that label.
- Missing environment label or missing environment configuration must fail closed deterministically.
- Engine must not perform fallback resolution, inferred mapping, profile substitution, or default profile merge.
- Fail-open behavior is prohibited. Any unresolved environment state must produce a deterministic non-allow outcome.

## 5. Single-Bundle Execution Model

- Exactly one bundle artifact is active per evaluation.
- No multi-bundle composition, layering, or overlay.
- No merge strategy between artifacts.
- If multiple bundle inputs are provided, execution fails before policy evaluation.
- Directory-of-bundles input is invalid and must be rejected before policy evaluation.
- Base-plus-override bundle patterns are invalid.
- Implicit search paths for locating additional bundles are invalid.

## 6. Software Layering and Dependency Boundaries

### Layers
- CLI: collects inputs, selects profile and bundle artifact path, invokes engine.
- Engine: orchestrates loading, evaluation, and evidence recording.
- Bundle Loader: validates bundle artifact format and manifest, exposes loaded policy/data to evaluator.
- OPA Evaluation: evaluates decision query against loaded bundle and runtime input.
- Evidence: records decision and immutable metadata.

### Allowed Dependency Direction
- CLI -> Engine -> Bundle Loader -> OPA Evaluation -> Evidence

### Forbidden Dependencies
- Evidence must not depend on OPA internals.
- Evidence must not import OPA packages.
- Bundle Loader must not import Engine packages.
- Bundle Loader must not depend on decision semantics.
- OPA Evaluation must not know filesystem layout or bundle path resolution.
- CLI must not parse bundle internals directly.
- Go runtime must not include environment-specific branching rules.
- Dependency direction is one-way and irreversible for this architecture baseline.

## 7. Determinism Model

- Determinism contract: same input, same environment label, same bundle revision, same decision query, same decision output.
- Bundle artifact is immutable once released.
- Mutable artifacts or re-tagged revisions are prohibited.
- Decision output ordering is required to be deterministic:
  - `violations` must be sorted by `rule_id` ascending, then by `message` ascending.
  - `rule_ids` must be sorted ascending.
  - `hints` must be sorted ascending after de-duplication.
- No decision field must not rely on map iteration order.
- Determinism requirements apply to both CLI and engine outputs.

## 8. Evidence Model Extension

Every decision record must include:
- `bundle_revision`
- `profile_name`
- `environment_label`
- `input_hash`

Rationale:
- `bundle_revision` binds decision to exact policy artifact.
- `profile_name` identifies policy family without inferring from filesystem.
- `environment_label` preserves execution context while remaining opaque.
- `input_hash` binds decision to exact input content.

## 9. Migration Strategy

### Move Inline Rules to Bundle Layout
- Consolidate policy modules under policy roots with manifest ownership.
- Remove ad-hoc profile loading that bypasses bundle packaging.

### Move Hardcoded Thresholds to Data Namespace
- Eliminate Go constants and inline Rego literals for thresholds.
- Store all numeric and categorical controls in bundle data documents.

### Migration Phases
- Phase 1: Dual-read validation in non-production checks (current source vs built bundle).
- Phase 2: Bundle-only runtime path.
- Phase 3: Remove legacy non-bundle loading path.

## 10. Acceptance Criteria

- Runtime accepts only standard OPA bundle artifacts for policy execution.
- Manifest validation gate is mandatory: missing/invalid `.manifest` is a hard failure before policy evaluation.
- Namespace validation gate is mandatory: bundle contents outside declared policy roots are a hard failure.
- Exactly one bundle artifact is accepted per execution; multiple inputs or bundle directories are hard failures.
- Static scan gate verifies no Go environment branching in evaluation path.
- Static scan gate verifies no Rego environment literals in decision rules.
- Static scan gate verifies thresholds are data-driven and not hardcoded in Go/Rego policy logic.
- Evidence record must contain non-empty `bundle_revision`, `profile_name`, `environment_label`, and `input_hash`.
- Determinism gate:
  - repeated evaluation of identical input/environment/bundle_revision must yield byte-identical ordered outputs for `violations`, `rule_ids`, and `hints`.
  - any divergence is a test failure.

## 11. Refinement Summary

### Removed Ambiguities
- Removed ambiguity between `bundle_revision` and `policy_ref`; authority is now explicit.
- Removed ambiguity for missing environment behavior; fail-closed is now explicit.
- Removed ambiguity for bundle discovery; implicit search and bundle directories are explicitly invalid.
- Removed ambiguity in output ordering; sorting and map-order prohibition are explicit.

### Clarified Invariants
- Bundle revision identity is authoritative and immutable.
- Engine does not inject defaults and does not branch by environment.
- Single-bundle execution is absolute and pre-evaluation enforced.
- Dependency boundaries are strict and one-way.
