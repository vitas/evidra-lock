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
    …
  ],
  "actor": {
    "type": "human" | "agent"
  },
  "source": "cli" | "mcp"
}
```

- `actions[].kind` is a stable tuple such as `terraform.plan` or `kubectl.delete`; policies match on kind to know which action is under evaluation.
- `actions[].payload` carries the tool’s details (Terraform plans export `destroy_count`, `planned_values`, etc.; Kubernetes actions embed `namespace`, manifests, and delete counts). Rules can `object.get` fields instead of assuming full schemas.
- `actions[].risk_tags` mirrors the tags set by the adapter (e.g., `breakglass`, `change-approved`, `approved_public`). Deny rules typically inspect these to allow exceptions.
- `actor.type` records who triggered the run; alerts like autonomous execution rely on `agent`.
- `source` distinguishes CLI runs from MCP/agent runs so policies can honor different guardrails.

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
  "risk_level": "normal" | "high",
  "reason": string,
  "reasons": [string],
  "hits": [string],
  "hints": [string],
}
```

- `allow` drives enforcement: true permits the invocation, false denies it.
- `risk_level` is `"high"` whenever a deny fires or if the input carried a `"breakglass"` tag; otherwise it stays `"normal"`.
- `reason` remains the legacy single-string summary (first deny message) that many adapters log.
- `reasons` is the ordered list of all deny messages emitted during evaluation.
- `hits` enumerates stable rule IDs (`POL-*` or `WARN-*`) whose `deny`/`warn` bodies matched.
- `hints` are the remediation instructions pulled from `data.rule_hints` for each hit; warn rules may contribute optional hints as well.

## 3. Rule Structure

Add rules inside `policy/profiles/ops-v0.1/policy/rules/`, one file per intent.

- Use `package evidra.policy`.
- Emit denies via `deny["POL-XXX-YY"] = "message"` or warnings via `warn["WARN-XXX-YY"] = "message"`.
- IDs must stay stable once released; changing a label breaks downstream evidence and CLI hints.
- Keep each rule focused (<40 lines) and avoid object.get-heavy decision building—push logic into helper functions or refer to `policy/defaults.rego`.

Example deny skeleton:

```rego
package evidra.policy

import data.evidra.policy.defaults as defaults

deny["POL-EXAMPLE-01"] = msg if {
  some action := input.actions[_]
  defaults.action_namespace(action) == "prod"
  not defaults.has_tag(action, "change-approved")
  msg := "Production changes require change-approved"
}
```

## 4. Remediation Hints

- Hints live in `policy/profiles/ops-v0.1/data.json` under `rule_hints`.
- Keys must match the rule IDs emitted by the policy (not the human-readable messages).
- Each rule should map to 1–3 actionable hints such as `"Add risk_tag: breakglass"` rather than abstract explanations.
- When a rule hits, the decision aggregator dedupes and sorts hints to give operators a concise next step.

## 5. Testing Policy

- Run `opa test policy/profiles/ops-v0.1` from the repo root to exercise all policy rules/tests.
- Run `go test ./pkg/policy ./pkg/validate ./pkg/scenario` to verify Go integration with the policy decision contract.
- Use `evidra validate <fixtures>` (or `examples/*.json`) to see the CLI print hits/hints and confirm evidence contains the expected fields.

## Single Source of Truth

The structured profile under `policy/profiles/ops-v0.1` (shim + `policy/` directory + `data.json`) is the single source of truth. Do not edit the old bundles-based policies/documentation; edit this profile instead. policy tests and runtime integration already point to this directory.
