# Evidra Rule ID Redesign Specification (Design-Only)

## Section 1 — Rule ID Naming Standard

### 1.1 Normative Format
- Rule identifiers MUST use the format `domain.invariant_name`.
- `domain` MUST be lowercase ASCII letters and digits only.
- `invariant_name` MUST be lowercase snake_case.
- Rule identifiers MUST be dot-separated with exactly one dot.
- Rule identifiers MUST be stable across versions once released.

### 1.2 Normative Semantics
- Rule identifiers MUST describe the invariant that is violated or flagged.
- Rule identifiers MUST NOT encode environment (`prod`, `dev`, `stage`, `qa`, or equivalents).
- Rule identifiers MUST NOT encode decision severity (`deny`, `warn`, `allow`, `block`, `advisory`).
- Rule identifiers MUST NOT encode ordinals (`01`, `02`, etc.).
- Rule identifiers MUST NOT encode policy version.

### 1.3 Valid and Invalid Forms
Valid:
- `terraform.public_exposure_without_approval`
- `k8s.protected_namespace_without_breakglass`
- `ops.mass_delete_threshold_exceeded`
- `ops.autonomous_execution_requires_review`

Invalid:
- `POL-PROD-01` (encodes severity family + ordinal + environment context)
- `WARN-AUTO-01` (encodes severity + ordinal)
- `ops.prod_change_without_approval` (encodes environment)
- `k8s.mass-delete` (not snake_case)
- `ops.mass.delete` (more than one dot)

## Section 2 — Allowed Domains (Taxonomy)

### 2.1 Controlled Domain Vocabulary
- `ops`
  - Cross-cutting operational invariants that span tools or represent policy-system invariants.
  - Examples: change approval, breakglass governance, synthetic policy-contract guards.
- `terraform`
  - Invariants derived from Terraform plan semantics.
  - Examples: public exposure, large-scale destroy/delete effects.
- `k8s`
  - Invariants derived from Kubernetes manifests/actions.
  - Examples: protected namespace controls, privileged configuration checks.
- `argocd`
  - Invariants derived from Argo CD application and sync behaviors.
- `helm`
  - Invariants derived from Helm chart/rendered output semantics.
- `aws-s3`
  - Invariants derived from object storage access posture.
- `aws-iam`
  - Invariants derived from identity and access control posture.

### 2.2 Domain Admission Rules
- A new domain MUST represent a distinct technical system boundary.
- A new domain MUST have at least one concrete invariant.
- A new domain MUST NOT duplicate semantics already covered by an existing domain.
- Domain introduction MUST include explicit rationale and ownership.

## Section 3 — Complete Mapping Table

### 3.1 Inventory Result by Source
- Rego rules (active runtime IDs):
  - `POL-KUBE-01`, `POL-PROD-01`, `POL-PUB-01`, `POL-DEL-01`, `WARN-AUTO-01`, `WARN-BREAKGLASS-01`.
- Go constants:
  - No dedicated rule-ID constants exist.
- Go runtime literal (active synthetic fallback ID):
  - `POL-UNLABELED-01`.
- Tests (assert active runtime IDs):
  - `POL-KUBE-01`, `POL-PROD-01`, `POL-PUB-01`, `POL-DEL-01`, `WARN-AUTO-01`, `WARN-BREAKGLASS-01`.
- Docs (concrete + template/example IDs):
  - Concrete: `POL-PROD-01`, `POL-EXAMPLE-01`.
  - Templates: `POL-XXX-YY`, `WARN-XXX-YY`.

### 3.2 Mapping (Concrete IDs)

| old_id | new_rule_id | rationale |
|---|---|---|
| `POL-KUBE-01` | `k8s.protected_namespace_without_breakglass` | Captures invariant breach (protected namespace change lacking breakglass) without severity, environment, or ordinal. |
| `POL-PROD-01` | `ops.change_without_approval` | Captures approval invariant breach without encoding environment label. |
| `POL-PUB-01` | `terraform.public_exposure_without_approval` | Captures Terraform-specific exposure invariant breach without severity encoding. |
| `POL-DEL-01` | `ops.mass_delete_threshold_exceeded` | Captures threshold invariant breach across tools; stable and severity-neutral. |
| `WARN-AUTO-01` | `ops.autonomous_execution_requires_review` | Captures governance invariant for autonomous execution without warn/deny encoding. |
| `WARN-BREAKGLASS-01` | `ops.breakglass_usage_requires_review` | Captures governance invariant around breakglass use without severity encoding. |
| `POL-UNLABELED-01` | `ops.deny_without_rule_label` | Captures policy-contract invariant failure (deny emitted without label) as synthetic guardrail ID. |
| `POL-EXAMPLE-01` | `ops.example_invariant` | Documentation example remapped to canonical format. Not a runtime rule. |

### 3.3 Explicit Ambiguities and Resolution
- `POL-PROD-01` currently fires on `prod` namespace only, but the redesigned ID removes environment encoding by rule.
  - Resolution: use invariant-centric `ops.change_without_approval`; environment-specific scope remains policy logic, not ID.
- `POL-UNLABELED-01` is synthetic (Go fallback), not emitted by Rego rules.
  - Resolution: keep as `ops.deny_without_rule_label` and classify as policy-contract guardrail ID.
- `POL-XXX-YY` and `WARN-XXX-YY` are placeholders/templates, not concrete rule IDs.
  - Resolution: not mapped as runtime IDs; replace all templates with `domain.invariant_name` guidance text.

### 3.4 Collision and Duplication Status
- Proposed `new_rule_id` set has zero duplicates.
- Proposed `new_rule_id` set has zero collisions.
- Each concrete old ID maps to exactly one new ID.

## Section 4 — Consistency Checklist

### 4.1 Collision Check
- No two old IDs map to the same new ID.
- No new ID duplicates another new ID.

### 4.2 Naming Clarity Check
- Every new ID reads as an invariant, not a workflow state.
- Every new ID is understandable without severity prefixes.

### 4.3 Invariant Stability Check
- New IDs do not embed environment, severity, or ordinals.
- New IDs remain stable if severity policy changes (warn/deny behavior changes).

### 4.4 Scope Correctness Check
- Tool-specific invariants use specific domains (`terraform`, `k8s`).
- Cross-tool and policy-contract invariants use `ops`.

### 4.5 Domain Correctness Check
- Domain reflects origin of invariant semantics.
- No domain leakage (for example, Terraform-only behavior under `k8s`).

## Section 5 — Decision Log

### 5.1 Why `domain.invariant_name` Was Chosen
- It decouples identity from enforcement outcome.
- It preserves readability and machine stability simultaneously.
- It scales without ordinal exhaustion and without renumbering.

### 5.2 Why Environment and Severity Were Removed
- Environment in IDs creates brittle identifiers that change with deployment taxonomy.
- Severity in IDs couples stable identity to mutable policy posture.
- Removing both ensures IDs track invariant meaning only.

### 5.3 Tradeoffs Considered
- Keeping legacy `POL-*` / `WARN-*` style was rejected due to embedded severity class and ordinal churn.
- Embedding environment in IDs was rejected due to explicit non-requirement and stability risk.
- Using only broad `ops.*` IDs was rejected where tool-specific semantics are clearer (`terraform.*`, `k8s.*`).

### 5.4 Why This Is Future-Proof
- The scheme supports adding new invariants without renaming existing IDs.
- The scheme supports changing enforcement level without changing IDs.
- The scheme supports expansion into additional technical domains without namespace collision.
