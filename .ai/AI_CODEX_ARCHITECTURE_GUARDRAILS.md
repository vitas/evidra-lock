# AI Architecture Guardrails: OPA Bundle Standard

## 1. Architectural Invariants

- OPA bundle artifact is the only accepted policy artifact at runtime.
- Exactly one bundle artifact is evaluated per execution.
- Environment is treated strictly as an opaque string label.
- Go runtime contains no environment-specific branching logic.
- Go runtime never injects a default environment label.
- Rego rules contain no hardcoded environment literals.
- All thresholds and policy configuration are data-driven under policy roots.
- Evidence must bind decision to `bundle_revision` and `input_hash`.
- Decision ordering is deterministic for `violations`, `rule_ids`, and `hints`.
- Decision construction must not rely on map iteration order.

## 2. Prohibited Patterns

- Hardcoded thresholds in Go or Rego.
- Inline environment checks tied to named environments.
- Silent fallback to default profile or default bundle artifact when resolution fails.
- Silent fallback when environment label is missing or unresolved.
- Automatic bundle merging, layering, or precedence chains.
- Runtime composition of multiple bundle artifacts.
- Multiple bundle inputs in one execution context.
- Directory-of-bundles input for evaluation.
- Implicit search paths for bundle discovery.
- Policy logic split between bundle data and hidden runtime constants.

## 3. CI Enforcement Strategy

### Static Policy Guardrails
- Reject Rego changes that introduce environment literals in decision rules.
- Reject threshold literals in rule logic when equivalent data keys are expected.
- Enforce manifest presence and required fields in bundle artifacts.
- Enforce namespace ownership against declared policy roots (manifest roots).
- Enforce deterministic ordering for `violations`, `rule_ids`, and `hints`.
- Enforce prohibition on map-order-dependent output generation.

### Review Checklist
- Does change preserve single-bundle-artifact execution?
- Are all policy controls sourced from data namespace?
- Is environment still opaque and unbranched in Go?
- Is evidence metadata sufficient to replay decision provenance?
- Does release metadata match manifest revision, profile, and artifact identity?
- Does change preserve fail-closed behavior for unresolved environment configuration?
- Does change avoid introducing implicit bundle lookup or fallback behavior?

### Build and Validation Gates
- Bundle validity gate (manifest + roots + loadability).
- Determinism gate (rebuild checksum consistency).
- Decision contract gate (allow/risk/reason determinism under fixed input/revision/env).
- No-env-branch static scan gate (Go evaluation path).
- Namespace validation gate (all data keys under declared policy roots).
- All guardrail violations are release-blocking.

## 4. Drift Detection Model

### Detection Signals
- New runtime branches conditioned on environment values.
- New constants used as policy thresholds outside bundle data.
- Evidence records missing `bundle_revision` or `input_hash`.
- Release artifacts whose manifest revision does not match release metadata.
- Decision arrays emitted in non-deterministic order across repeated runs.
- New fallback paths for unresolved bundle or environment inputs.

### Detection Cadence
- Pull-request validation for static and structural checks.
- Release-time validation for bundle integrity and reproducibility.
- Scheduled repository audits for forbidden patterns and stale exceptions.
- Exception expiration checks run in pull-request and release workflows.

### Remediation Policy
- Architectural drift is release-blocking until corrected.
- Temporary exceptions require explicit owner, explicit reason, explicit expiration date, and tracking issue.
- Temporary exceptions must be time-boxed and automatically fail once expired.
- Expired exceptions are treated as release-blocking policy violations.

## 5. Refinement Summary

### Removed Ambiguities
- Removed ambiguity on environment fallback behavior by explicitly prohibiting default injection and fallback.
- Removed ambiguity on multi-bundle behavior by explicitly prohibiting multiple inputs and bundle directories.
- Removed ambiguity on decision determinism by requiring sorted arrays and prohibiting map-order reliance.

### Clarified Invariants
- Fail-closed behavior is mandatory for unresolved environment configuration.
- Guardrail violations are release-blocking.
- Exception handling is explicit, time-boxed, and auto-enforced.
