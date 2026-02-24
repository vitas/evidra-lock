# Evidra Policy Contract (ops-v0.1)

## 1. Input Schema

Policy always receives a canonical `input` object derived from the ToolInvocation details that the runtime supplies.

```
input = {
  "actions": [
    {
      "kind": "terraform.plan" | "kubectl.delete" | "k8s.apply" | ...,
      "payload": { ... },          # tool-specific metadata
      "risk_tags": ["breakglass", "change-approved", ...]
    },
    ...
  ],
  "actor": {
    "type": "human" | "agent"
  },
  "source": "cli" | "mcp",
  "environment": "production" | "staging" | ...   # opaque label, optional
}
```

- `actions[].kind` is a stable tuple such as `terraform.plan` or `kubectl.delete`; policies match on kind to know which action is under evaluation.
- `actions[].payload` carries the tool's details (Terraform plans export `destroy_count`, `planned_values`, etc.; Kubernetes actions embed `namespace`, manifests, and delete counts). Rules can `object.get` fields instead of assuming full schemas.
- `actions[].risk_tags` mirrors the tags set by the adapter (e.g., `breakglass`, `change-approved`, `approved_public`). Deny rules typically inspect these to allow exceptions.
- `actor.type` records who triggered the run; alerts like autonomous execution rely on `agent`.
- `source` distinguishes CLI runs from MCP/agent runs so policies can honor different guardrails.
- `environment` is an opaque label forwarded to OPA; rules use it to resolve environment-specific params via `resolve_param`/`resolve_list_param` helpers.

Minimal plan input:

```
{
  "actions": [
    {
      "kind": "terraform.plan",
      "payload": {
        "destroy_count": 8,
        "planned_values": {...},
        "publicly_exposed": true
      },
      "risk_tags": []
    }
  ],
  "actor": {"type": "human"},
  "source": "cli"
}
```

Minimal manifest input:

```
{
  "actions": [
    {
      "kind": "kubectl.delete",
      "payload": {
        "namespace": "prod",
        "resource_count": 12
      },
      "risk_tags": ["breakglass"]
    }
  ],
  "actor": {"type": "agent"},
  "source": "mcp"
}
```

## 2. Decision Output Schema

Policies must publish a decision object under `data.evidra.policy.decision`. The Go runtime expects exactly:

```
decision = {
  "allow": bool,
  "risk_level": "low" | "medium" | "high",
  "reason": string,
  "reasons": [string],
  "hits": [string],
  "hints": [string],
}
```

- `allow` drives enforcement: true permits the invocation, false denies it.
- `risk_level` is `"high"` whenever a deny fires, `"medium"` when a breakglass tag is present, and `"low"` otherwise. The `low`/`medium` labels mark allowed flows, while `high` flags decisions that should be blocked or reviewed.
- `reason` remains the single-string summary (first deny message) that many adapters log.
- `reasons` is the ordered list of all deny messages emitted during evaluation.
- `hits` enumerates stable rule IDs (canonical `domain.invariant_name` format) whose `deny`/`warn` bodies matched.
- `hints` are the remediation instructions pulled from `data.evidra.data.rule_hints` for each hit; warn rules may contribute optional hints as well.

## 3. Rule Structure

Add rules inside `policy/bundles/ops-v0.1/evidra/policy/rules/`, one file per intent.

- Use `package evidra.policy`.
- Emit denies via `deny["domain.invariant_name"] = "message"` or warnings via `warn["domain.invariant_name"] = "message"`.
- Rule IDs use canonical `domain.invariant_name` format (e.g., `k8s.protected_namespace`, `ops.mass_delete`). See `ai/AI_RULE_ID_NAMING_STANDARD.md` for the full naming convention.
- IDs must stay stable once released; changing a label breaks downstream evidence and CLI hints.
- Use `resolve_param` / `resolve_list_param` helpers from `defaults.rego` to look up data-driven thresholds and lists instead of hardcoding values.
- Keep each rule focused (<40 lines) and avoid object.get-heavy decision building.

Example deny skeleton:

```rego
package evidra.policy

import data.evidra.policy.defaults as defaults

deny["ops.unapproved_change"] = msg if {
  some action := input.actions[_]
  protected := defaults.resolve_list_param("k8s.namespaces.protected")
  defaults.action_namespace(action) == protected[_]
  not defaults.has_tag(action, "change-approved")
  msg := "Changes in protected namespace require change-approved tag"
}
```

## 4. Data-Driven Params

Policy parameters live in OPA bundle data files:
- `policy/bundles/ops-v0.1/evidra/data/params/data.json` — thresholds and lists, keyed by canonical param names with `by_env` maps for environment-specific values.
- `policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json` — remediation hints keyed by canonical rule IDs.

Param structure example:
```json
{
  "ops.mass_delete.max_deletes": {
    "by_env": { "default": 5, "staging": 10 }
  },
  "k8s.namespaces.restricted": {
    "by_env": { "default": ["kube-system"] }
  }
}
```

The `resolve_param(key)` and `resolve_list_param(key)` helpers in `defaults.rego` look up the param by key, check for an environment-specific value via `input.environment`, and fall back to `"default"`.

## 5. Remediation Hints

- Hints live in `policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json`.
- Keys must match the canonical rule IDs emitted by the policy (e.g., `k8s.protected_namespace`).
- Each rule should map to 1-3 actionable hints such as `"Add risk_tag: breakglass"` rather than abstract explanations.
- When a rule hits, the decision aggregator dedupes and sorts hints to give operators a concise next step.

## 6. Testing Policy

- Run `opa test policy/bundles/ops-v0.1/ -v` from the repo root to exercise all policy rules/tests.
- Run `go test ./pkg/policy ./pkg/validate ./pkg/scenario` to verify Go integration with the policy decision contract.
- Use `evidra validate <fixtures>` (or `examples/*.json`) to see the CLI print hits/hints and confirm evidence contains the expected fields.

## 7. Bundle Structure

The OPA bundle under `policy/bundles/ops-v0.1/` follows OPA's native bundle layout:

```
policy/bundles/ops-v0.1/
+-- .manifest                          # revision, roots, profile_name
+-- evidra/
|   +-- policy/
|   |   +-- policy.rego                # shim: exports data.evidra.policy.decision
|   |   +-- decision.rego              # aggregator combining deny/warn results
|   |   +-- defaults.rego              # helpers: resolve_param, has_tag, action_namespace
|   |   +-- rules/
|   |       +-- deny_kube_system.rego
|   |       +-- deny_prod_without_approval.rego
|   |       +-- deny_public_exposure.rego
|   |       +-- deny_mass_delete.rego
|   |       +-- warn_breakglass.rego
|   |       +-- warn_autonomous_execution.rego
|   +-- data/
|       +-- params/data.json           # maps to data.evidra.data.params
|       +-- rule_hints/data.json       # maps to data.evidra.data.rule_hints
+-- tests/                             # OPA tests (outside bundle archive)
```

The `.manifest` file declares the bundle revision, OPA roots, and profile name. Directory paths under `evidra/data/` determine the OPA data namespace (`data.evidra.data.params`, `data.evidra.data.rule_hints`).

## Single Source of Truth

The OPA bundle under `policy/bundles/ops-v0.1` is the single source of truth. Edit this bundle directly — policy tests and runtime integration already point to these files.
