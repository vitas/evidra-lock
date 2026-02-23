---

# Policy Contract v0.1

This document defines the strict interface between Evidra Core and the Policy Engine (OPA/Rego). The policy engine evaluates each ToolInvocation and must remain deterministic, side-effect free, and focused on decision-making.

---

## 1. Evaluation Model

- Default decision: allow unless deny rules match.
- Policy evaluation must be pure (no external side effects).
- Policy must not execute tools or mutate input.

---

## 2. Input Schema

OPA receives the following canonical `input` object:

```
{
  "actions": [
    {
      "kind": "terraform.plan" | "kubectl.delete" | "k8s.apply" | ..., 
      "payload": { ... },                # tool-specific metadata (namespace, destroy_count, exposures, etc.)
      "risk_tags": ["breakglass", "change-approved", ...]
    }
  ],
  "actor": {
    "type": "human" | "agent"
  },
  "source": "cli" | "mcp"
}
```

### Field Definitions

- `actions[].kind`: normalized tool+operation (e.g., `terraform.plan`). Use it to distinguish Terraform vs. Kubernetes semantics.
- `actions[].payload`: contains observables such as `destroy_count`, `publicly_exposed`, or `namespace`. Access safely with `object.get`.
- `actions[].risk_tags`: tags propagated by the adapter; high-risk rules typically require `breakglass`, `change-approved`, or `approved_public` tags before allowing them.
- `actor.type`: identifies whether a human (`"human"`) or an agent (`"agent"`) triggered the run.
- `source`: surface name (`"cli"` or `"mcp"`) so policies can apply stricter checks for MCP/agent traffic.

Minimal Terraform example:

```
{
  "actions": [
    {
      "kind": "terraform.plan",
      "payload": {
        "destroy_count": 8,
        "publicly_exposed": true
      },
      "risk_tags": []
    }
  ],
  "actor": {"type": "human"},
  "source": "cli"
}
```

Minimal Kubernetes example:

```
{
  "actions": [
    {
      "kind": "kubectl.delete",
      "payload": {
        "namespace": "kube-system",
        "resource_count": 12
      },
      "risk_tags": []
    }
  ],
  "actor": {"type": "agent"},
  "source": "mcp"
}
```

---

## 3. Decision Output Schema

Policies must publish `data.evidra.policy.decision` with the structure:

```
{
  "allow": bool,
  "risk_level": "normal" | "high",
  "reason": string,
  "reasons": [string],
  "hits": [string],
  "hints": [string]
}
```

### Field Requirements

- `allow`: final enforcement decision. True = permitted, false = denied.
- `risk_level`: `"high"` whenever a deny fires or a `breakglass` tag exists; `"normal"` otherwise.
- `reason`: the first deny message (legacy single-string summary).
- `reasons`: the ordered list of all deny messages produced during evaluation.
- `hits`: stable rule IDs (`POL-*` for denies, `WARN-*` for warnings) that inform evidence and CLI output.
- `hints`: remediation guidance pulled from `data.rule_hints` corresponding to each rule ID; duplicates are deduped before presentation.

Evidence and CLI tooling expect these fields to render PASS/FAIL outcomes, rule IDs, and actionable hints.

---

## 4. Rule Style Guide

- Place small rules in `policy/profiles/ops-v0.1/policy/rules/`.
- Use `package evidra.policy`.
- Define denies via `deny["POL-XXX-YY"] = "message" if { ... }` and warnings via `warn["WARN-XXX-YY"] = "message" if { ... }`.
- Rule IDs must remain stable once released; renaming a label invalidates historical evidence.
- Keep each rule file <40 non-empty lines. Move shared helpers into `policy/defaults.rego`.

Example:

```
package evidra.policy

import data.evidra.policy.defaults as defaults

deny["POL-PROD-01"] = msg if {
  some action := input.actions[_]
  defaults.action_namespace(action) == "prod"
  not defaults.has_tag(action, "change-approved")
  msg := "Production changes require change-approved"
}
```

---

## 5. Remediation Hints

- Define hints in `policy/profiles/ops-v0.1/data.json` inside the `rule_hints` object.
- The map keys must match rule IDs (e.g., `"POL-PROD-01"`).
- Each rule should list 1–3 actionable hints such as "Add risk_tag: breakglass".
- Hints describe what to change, not why the input failed.
- The decision aggregator dedupes hints per evaluation and exposes them via CLI/evidence.

---

## 6. Testing

- Run `opa test policy/profiles/ops-v0.1` for policy/unit tests.
- `go test ./...` covers Go integration with the structured policy, runtime, and evidence layers.
- `evidra validate bundles/ops/examples/*.json` exercises CLI output, hits, hints, and evidence creation.

---

## 7. Single Source of Truth

`policy/profiles/ops-v0.1` (shim + `policy/` directory + `data.json`) is the single authoritative profile. The runtime, bundles, and CLI default to the files in this directory when no overrides are supplied via `--policy`/`--data` or `EVIDRA_POLICY_PATH`/`EVIDRA_DATA_PATH`. No other directories should be treated as authoritative.
