# Policy Technical Specification — ops-v0.1

Bundle revision: `ops-v0.1.0-dev`
Last updated: 2026-02-25

This document is the normative engineering contract for the `ops-v0.1` policy bundle. It defines the input/output schemas, evaluation semantics, rule authoring constraints, bundle layout, and testing requirements. Implementations MUST conform to this specification. For human-readable rule descriptions and examples, see [POLICY_CATALOG.md](POLICY_CATALOG.md).

---

## 1. Canonical Input Schema

Policy evaluation MUST receive a canonical `input` object. The authoritative structure is:

```json
{
  "actions": [
    {
      "kind": "<domain>.<verb>",
      "payload": {},
      "risk_tags": ["<tag>", "..."]
    }
  ],
  "actor": {
    "type": "human | agent"
  },
  "source": "cli | mcp",
  "environment": "<opaque-string>"
}
```

Field semantics:

- `actions` — Required. Array of one or more action objects representing the operations under evaluation.
- `actions[].kind` — Required. A stable two-segment string (`domain.verb`) identifying the action type. Rules match on `kind` to select which actions to evaluate. Known kinds: `terraform.plan`, `kubectl.delete`, `kubectl.apply`, `k8s.apply`. Rules MUST use `object.get` rather than direct field access when payload fields may be absent.
- `actions[].payload` — Required (may be empty object). Tool-specific metadata. Rules extract only the fields they need; the schema is not fixed across kinds.
- `actions[].risk_tags` — Required (may be empty array). Tags asserted by the caller. Deny rules inspect tags to evaluate bypass conditions. Known tags: `breakglass`, `change-approved`, `approved_public`.
- `actor.type` — Optional. Identifies the request originator. Used by `ops.autonomous_execution`. Known values: `"human"`, `"agent"`.
- `source` — Optional. Identifies the invocation path. Used by `ops.autonomous_execution`. Known values: `"cli"`, `"mcp"`.
- `environment` — Optional. Opaque string label used for environment-aware param resolution. See §4.

Minimal Terraform plan input:

```json
{
  "actions": [
    {
      "kind": "terraform.plan",
      "payload": { "destroy_count": 8, "publicly_exposed": true },
      "risk_tags": []
    }
  ],
  "actor": { "type": "human" },
  "source": "cli"
}
```

Minimal Kubernetes delete input:

```json
{
  "actions": [
    {
      "kind": "kubectl.delete",
      "payload": { "namespace": "prod", "resource_count": 12 },
      "risk_tags": ["breakglass"]
    }
  ],
  "actor": { "type": "agent" },
  "source": "mcp"
}
```

---

## 2. Decision Output Schema

Policy MUST publish a decision object at the OPA path `data.evidra.policy.decision`. The runtime expects exactly the following structure:

```json
{
  "allow": "<bool>",
  "risk_level": "low | medium | high",
  "reason": "<string>",
  "reasons": ["<string>"],
  "hits": ["<rule-id>"],
  "hints": ["<string>"]
}
```

Field semantics:

- `allow` — Boolean. `true` permits the invocation; `false` denies it. In enforce mode, `false` blocks execution. In observe mode, the value is recorded but does not block.
- `risk_level` — String enum. Computed from deny and tag state; see §3.
- `reason` — String. The first deny message, or `"ok"` when no denies fired. Provided for adapter compatibility; adapters that log a single summary line SHOULD use this field.
- `reasons` — Array of strings. All deny messages emitted during evaluation, in evaluation order.
- `hits` — Array of strings. Deduplicated, sorted canonical rule IDs (`domain.invariant_name`) whose `deny` or `warn` bodies matched. Populated by both deny and warn rules.
- `hints` — Array of strings. Deduplicated, sorted remediation strings collected from `data.evidra.data.rule_hints` for each label in `hits`. Warn rule hits contribute hints the same way deny rule hits do.

The decision object MUST always be present. All six fields MUST always be present, even when their values are empty arrays or default strings.

---

## 3. Risk Level Computation

`risk_level` is computed deterministically from two inputs: whether any deny rule fired, and whether any action carries the `breakglass` tag.

The algorithm, in priority order:

1. If `count(deny matches) > 0` → `risk_level = "high"`.
2. Else if any action in `input.actions` carries `"breakglass"` in `risk_tags` → `risk_level = "medium"`.
3. Else → `risk_level = "low"`.

The `"medium"` level marks allowed flows that used a breakglass bypass. The `"high"` level marks denied flows regardless of tags. The `"low"` level marks unconditionally clean flows.

A request that both fires a deny and carries `breakglass` evaluates to `"high"` (deny takes precedence).

---

## 4. Environment Semantics and Param Resolution

### Environment

`input.environment` is optional and MUST be treated as an opaque string label. The runtime MUST NOT validate it against an enumerated set. Its sole purpose is to index into `by_env` maps in param data. No rule body MUST reference an environment name as a literal string.

### Param resolution algorithm

`resolve_param(key)` and `resolve_list_param(key)` implement the following lookup chain, evaluated in order. The first matching branch is used:

1. `input.environment` is non-empty AND `data.evidra.data.params[key].by_env[input.environment]` exists → return that value.
2. `input.environment` is non-empty AND `data.evidra.data.params[key].by_env[input.environment]` does not exist → return `data.evidra.data.params[key].by_env["default"]`.
3. `input.environment` is absent or empty → return `data.evidra.data.params[key].by_env["default"]`.

`resolve_list_param` is an alias for `resolve_param`. It exists for readability when the expected type is a list. The lookup logic is identical.

### Invariants

- Every param MUST define `by_env.default`. A param without `by_env.default` causes `resolve_param` to be undefined, which causes the rule body referencing it to be undefined, which silently suppresses the deny or warn. This is a correctness failure.
- Rules MUST NOT access `data.evidra.data.params` directly. All param access MUST go through `resolve_param` or `resolve_list_param`.

---

## 5. Bundle Layout Contract

The `ops-v0.1` bundle MUST conform to OPA's native bundle layout. The authoritative directory structure:

```
policy/bundles/ops-v0.1/
├── .manifest
├── evidra/
│   ├── policy/
│   │   ├── policy.rego              — package evidra.policy; shim exporting data.evidra.policy.decision
│   │   ├── decision.rego            — package evidra.policy.decision_impl; aggregates deny/warn
│   │   ├── defaults.rego            — package evidra.policy.defaults; helpers
│   │   └── rules/
│   │       ├── deny_kube_system.rego
│   │       ├── deny_prod_without_approval.rego
│   │       ├── deny_public_exposure.rego
│   │       ├── deny_mass_delete.rego
│   │       ├── warn_breakglass.rego
│   │       └── warn_autonomous_execution.rego
│   └── data/
│       ├── params/data.json         — maps to data.evidra.data.params
│       └── rule_hints/data.json     — maps to data.evidra.data.rule_hints
└── tests/                           — OPA test suite (outside bundle archive)
```

### Data namespace mapping

| File path (relative to bundle root) | OPA data path |
|---|---|
| `evidra/data/params/data.json` | `data.evidra.data.params` |
| `evidra/data/rule_hints/data.json` | `data.evidra.data.rule_hints` |

OPA derives the data path from the directory structure under the bundle root. The `evidra/` segment maps to the `evidra` root declared in `.manifest`.

### `.manifest` requirements

The `.manifest` file MUST declare:
- `"roots": ["evidra"]` — confines the bundle to the `evidra` namespace.
- `"revision"` — a unique revision string per release.
- `"metadata.profile_name"` — identifies the policy profile; current value: `"ops-v0.1"`.

---

## 6. Rule Authoring Contract

### Package and rule structure

- All rules MUST use `package evidra.policy`.
- Deny rules MUST emit via `deny["<rule-id>"] = "<message>"`.
- Warn rules MUST emit via `warn["<rule-id>"] = "<message>"`.
- Each rule file MUST contain exactly one logical rule intent.
- Rule files MUST be placed under `evidra/policy/rules/`.
- Rule files MUST NOT exceed 40 lines of Rego.

### Rule ID constraints

- Format: `domain.invariant_name` — exactly two dot-separated segments, all lowercase.
- MUST NOT encode environment names (e.g., `ops.mass_delete.prod` is invalid).
- MUST NOT encode severity or risk level (e.g., `ops.high_mass_delete` is invalid).
- MUST NOT include version ordinals (e.g., `ops.mass_delete_v2` is invalid).
- MUST be stable after release. Changing a rule ID after release invalidates existing evidence records and breaks hint lookups.

### Param usage

- Rules MUST NOT hardcode tunable numeric or string values in rule bodies.
- Rules MUST retrieve configurable values via `resolve_param(key)` or `resolve_list_param(key)`.
- Rules MUST NOT access `data.evidra.data.params` directly.
- Rules MUST NOT reference `input.environment` directly to branch on environment.

### Helpers available

All helpers are defined in `defaults.rego` under `package evidra.policy.defaults`:

| Helper | Signature | Purpose |
|---|---|---|
| `has_tag` | `has_tag(action, tag) bool` | Returns true if `action.risk_tags` contains `tag`. Safe when `risk_tags` is absent. |
| `action_namespace` | `action_namespace(action) string` | Extracts namespace from `action.payload.namespace` or `action.target.namespace`. Undefined when neither is present. |
| `resolve_param` | `resolve_param(key) any` | Returns environment-aware param value; falls back to `by_env.default`. |
| `resolve_list_param` | `resolve_list_param(key) array` | Alias for `resolve_param`; use when result is a list. |

### Deny rule skeleton

```rego
package evidra.policy

import data.evidra.policy.defaults as defaults

deny["ops.example_rule"] = msg if {
  some i
  action := input.actions[i]
  threshold := defaults.resolve_param("ops.example_rule.threshold")
  action.payload.count > threshold
  not defaults.has_tag(action, "breakglass")
  msg := "Example rule message"
}
```

---

## 7. Hints Contract

- Hints MUST be stored in `evidra/data/rule_hints/data.json`.
- Keys MUST exactly match canonical rule IDs emitted by deny or warn rules.
- Each rule ID MUST map to an array of 1–3 short, actionable strings.
- Hint strings MUST describe a concrete corrective action, not an abstract explanation.
- The decision aggregator deduplicates and sorts hints across all hits before emitting them in the decision output. Rule authors MUST NOT assume a specific position for their hints.
- If a rule ID has no entry in `rule_hints`, no hint is emitted. This is valid but SHOULD be avoided for deny rules, as callers rely on hints to understand remediation.

---

## 8. Testing Contract

### OPA-level tests

- MUST be placed under `policy/bundles/ops-v0.1/tests/`.
- MUST use `package evidra.policy` and `import rego.v1`.
- Every deny rule MUST have at least one test that confirms the deny fires under trigger conditions and at least one test that confirms the deny does not fire when the bypass tag is present.
- Every warn rule MUST have at least one test confirming the warn fires and the resulting decision is `allow: true`.
- Tests MUST exercise the full decision path (`data.evidra.policy.decision with input as ...`) rather than individual rule predicates where possible.

Run command:

```
opa test policy/bundles/ops-v0.1/ -v
```

### Go integration tests

- `go test ./pkg/policy ./pkg/validate ./pkg/scenario` verifies the Go runtime's integration with the policy decision contract.
- The Go runtime expects the six-field decision schema defined in §2. Any change to the output schema MUST be accompanied by corresponding Go type updates.

Run command:

```
go test -race ./pkg/policy ./pkg/validate ./pkg/scenario
```

### CLI validation tests

- `evidra validate <fixture>` exercises the full evaluation pipeline end-to-end, including evidence recording.
- Fixture files in `examples/` serve as regression inputs. Adding a new fixture for each new rule trigger condition is recommended.

---

## 9. Determinism Expectations

The evaluation engine MUST produce deterministic outputs for identical inputs:

- `hits` MUST be sorted (lexicographic). The aggregator deduplicates and sorts via `sort()`.
- `hints` MUST be sorted (lexicographic). Same deduplication and sort applies.
- `reasons` MUST be stable in order (evaluation order of deny matches).
- `allow` MUST be `true` if and only if the set of deny matches is empty.
- `risk_level` MUST be computed from the deny count and breakglass tag state as defined in §3; it MUST NOT be influenced by any other input field.

---

## 10. Guardrail Invariants

The following invariants MUST hold at all times. Violations are correctness failures.

| # | Invariant |
|---|---|
| G1 | The OPA bundle at `policy/bundles/ops-v0.1/` is the single source of truth. No policy logic MUST exist outside this bundle. |
| G2 | Every param used by a rule MUST have a `by_env.default` entry in `params/data.json`. |
| G3 | Every rule MUST emit a stable, canonical rule ID in its deny or warn head. Unlabeled deny/warn rules are a correctness failure. |
| G4 | Rule bodies MUST NOT contain tunable literal values. All such values MUST come from `resolve_param`. |
| G5 | Rule bodies MUST NOT reference `input.environment` directly. |
| G6 | Rule IDs MUST NOT change after their first release into a stable bundle revision. |
| G7 | The `by_env` key `"default"` is reserved as the fallback key. It MUST NOT be used as the name of a real deployment environment. |
| G8 | Hints in `rule_hints/data.json` MUST be keyed by exactly the canonical rule ID emitted by the corresponding rule. Mismatched keys result in silent hint omission. |
| G9 | The decision output path `data.evidra.policy.decision` MUST always resolve to an object containing all six fields defined in §2. |
| G10 | `risk_level` MUST be computed from deny count and breakglass tag state only, as defined in §3. It MUST NOT be derived from `input.environment` or any param. |

---

## 11. External References

- Open Policy Agent: Rego Language Reference — for Rego syntax and built-in functions.
- OPA Playground — for interactive Rego development and testing.
- `opa test` command reference — for test runner flags and output format.
- `ai/AI_RULE_ID_NAMING_STANDARD.md` — authoritative rule ID naming conventions for this repository.
