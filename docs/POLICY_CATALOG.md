# Policy Catalog — ops-v0.1

Bundle revision: `ops-v0.1.0-dev`
Bundle root: `policy/bundles/ops-v0.1/`
Last updated: 2026-02-25

---

## 1. Overview

### What the policy bundle is

The `ops-v0.1` bundle is an OPA-native policy archive that contains all rule definitions, shared helpers, and their associated parameter and hint data. Every evaluation request is checked against this bundle; the bundle is the single source of truth for what is allowed, warned, or denied.

The bundle layout:

```
policy/bundles/ops-v0.1/
├── .manifest                    — revision, roots, profile metadata
├── evidra/policy/
│   ├── policy.rego              — exports the decision object
│   ├── decision.rego            — aggregates deny/warn results
│   ├── defaults.rego            — shared helpers (resolve_param, has_tag, action_namespace)
│   └── rules/                   — one file per rule
├── evidra/data/params/          — tunable parameter values
├── evidra/data/rule_hints/      — human-readable remediation hints
└── tests/                       — OPA test suite
```

For the full bundle contract, including data namespace mappings and OPA path conventions, see [POLICY_TECHNICAL_SPEC.md](POLICY_TECHNICAL_SPEC.md).

### What a rule ID is

A rule ID is a dotted canonical identifier of the form `domain.invariant_name`, for example `ops.mass_delete` or `k8s.protected_namespace`. Rule IDs:

- Appear in the `hits` array of every decision output.
- Are used as keys in the rule hints data to look up remediation strings.
- Must not encode environment names, severity levels, or ordinal counters.
- Are stable once released; changing an ID breaks downstream evidence records and hint lookups.

### What a hint is

A hint is a short, actionable remediation string tied to a rule ID. When a rule fires, its ID appears in `hits`. The decision aggregator collects hint strings for every entry in `hits`, deduplicates them, and returns them in the `hints` array of the decision output. Hints inform the caller what to do next; they do not affect the allow/deny outcome.

If a rule fires but its ID has no hint entry, no hint is emitted for it.

### How params work

Tunable values live in `evidra/data/params/data.json`. Each entry is keyed by a param identifier and holds a `by_env` map:

```json
"ops.mass_delete.max_deletes": {
  "by_env": { "default": 5, "production": 3 }
}
```

Rules read param values through the `resolve_param` and `resolve_list_param` helpers. These helpers are the only sanctioned way to access tunable values. No numeric or string literals that represent policy thresholds may appear in rule bodies.

Every param must define a `by_env.default` value. When no environment is supplied, or when the supplied environment has no matching entry, the `default` value is used automatically.

### How environment affects evaluation

`input.environment` is an optional, opaque string label. It is used solely to select an environment-specific value from a param's `by_env` map. When the environment is absent or does not match any key, the `default` entry is used. No environment names are hardcoded in rule logic.

For the precise lookup algorithm, see [POLICY_TECHNICAL_SPEC.md §4](POLICY_TECHNICAL_SPEC.md).

---

## 2. Quick Index

| Rule ID | Domain | Decision Type | Short Description | Key Params |
|---|---|---|---|---|
| `k8s.protected_namespace` | k8s | deny | Changes targeting a restricted namespace require a breakglass tag. | `k8s.namespaces.restricted` |
| `ops.mass_delete` | ops | deny | Bulk-delete actions exceeding a configurable threshold are blocked unless tagged breakglass. | `ops.mass_delete.max_deletes` |
| `ops.unapproved_change` | ops | deny | Changes targeting a protected namespace require a change-approved tag. | `k8s.namespaces.protected` |
| `ops.public_exposure` | ops | deny | Terraform plans that expose resources publicly require an approved_public tag. | _(none)_ |
| `ops.autonomous_execution` | ops | warn | Agent-originated requests via MCP are flagged for human review. | _(none)_ |
| `ops.breakglass_used` | ops | warn | Any action carrying the breakglass tag triggers a warning for audit purposes. | _(none)_ |

---

## 3. Rule Cards

---

### k8s.protected_namespace

**Intent**

Enforces that no action may target a namespace on the restricted list unless the action carries the `breakglass` risk tag. The restricted namespace list is configurable; the default value is `["kube-system"]`. This rule prevents unguarded writes to system-critical Kubernetes namespaces.

**Applies to**

Actions of any kind that carry a `namespace` field in `payload` or `target`. Confirmed in tests with `k8s.apply` actions targeting `kube-system`.

**Trigger**

Fires when at least one action has a namespace matching the restricted list and that action does not carry the `breakglass` risk tag.

**Outputs**

- Adds `"k8s.protected_namespace"` to `hits`.
- Adds `"Changes in restricted namespace require breakglass"` to `reasons`.
- Sets `allow` to `false`.
- Includes hints for `k8s.protected_namespace` in the `hints` array.

**Params**

| Param Key | Type | Default | Environment-aware |
|---|---|---|---|
| `k8s.namespaces.restricted` | list of strings | `["kube-system"]` | yes |

**Hints**

- `Add risk_tag: breakglass`
- `Or apply changes outside kube-system`

**Examples**

Denied — targets `kube-system` without breakglass:

```json
{
  "environment": "dev",
  "actions": [
    { "kind": "k8s.apply", "risk_tags": [], "payload": { "namespace": "kube-system" } }
  ]
}
```

Allowed — carries `breakglass` tag:

```json
{
  "environment": "dev",
  "actions": [
    { "kind": "k8s.apply", "risk_tags": ["breakglass"], "payload": { "namespace": "kube-system" } }
  ]
}
```

---

### ops.mass_delete

**Intent**

Enforces an upper bound on the number of resources deleted or destroyed in a single operation. The threshold is configurable per environment. `kubectl.delete` actions are checked against `payload.resource_count`; `terraform.plan` actions are checked against `payload.destroy_count`. Any action exceeding the threshold without a `breakglass` tag is denied.

**Applies to**

- `kubectl.delete` — reads `payload.resource_count`.
- `terraform.plan` — reads `payload.destroy_count` (treated as `0` when absent).

**Trigger**

Fires when at least one action of an applicable kind carries a count that exceeds the configured threshold and does not carry the `breakglass` risk tag.

**Outputs**

- Adds `"ops.mass_delete"` to `hits`.
- Adds `"Mass delete actions exceed threshold"` to `reasons`.
- Sets `allow` to `false`.
- Includes hints for `ops.mass_delete` in the `hints` array.

**Params**

| Param Key | Type | Default | Environment-aware |
|---|---|---|---|
| `ops.mass_delete.max_deletes` | number | `5` | yes (`production` → `3`) |

**Hints**

- `Reduce deletion scope`
- `Or add risk_tag: breakglass`

**Examples**

Denied — 12 resources deleted, default threshold is 5:

```json
{
  "environment": "dev",
  "actions": [
    { "kind": "kubectl.delete", "risk_tags": [], "payload": { "resource_count": 12 } }
  ]
}
```

Denied — 4 resources in `production` (threshold 3):

```json
{
  "environment": "production",
  "actions": [
    { "kind": "kubectl.delete", "risk_tags": [], "payload": { "resource_count": 4 } }
  ]
}
```

Allowed — same 4 resources in `dev` (threshold 5):

```json
{
  "environment": "dev",
  "actions": [
    { "kind": "kubectl.delete", "risk_tags": [], "payload": { "resource_count": 4 } }
  ]
}
```

---

### ops.unapproved_change

**Intent**

Enforces that changes targeting a namespace on the protected list carry a `change-approved` risk tag. The protected namespace list is configurable; the default value is `["prod"]`. This rule requires formal change-approval evidence for operations in designated protected environments.

**Applies to**

Actions of any kind that carry a `namespace` field in `payload` or `target`. Confirmed in tests with `k8s.apply` actions targeting the `prod` namespace.

**Trigger**

Fires when at least one action has a namespace matching the protected list and that action does not carry the `change-approved` risk tag.

**Outputs**

- Adds `"ops.unapproved_change"` to `hits`.
- Adds `"Production changes require change-approved"` to `reasons`.
- Sets `allow` to `false`.
- Includes hints for `ops.unapproved_change` in the `hints` array.

**Params**

| Param Key | Type | Default | Environment-aware |
|---|---|---|---|
| `k8s.namespaces.protected` | list of strings | `["prod"]` | yes |

**Hints**

- `Add risk_tag: change-approved`
- `Or run in observe mode`

**Examples**

Denied — targets `prod` without `change-approved`:

```json
{
  "environment": "prod",
  "actions": [
    { "kind": "k8s.apply", "risk_tags": [], "payload": { "namespace": "prod" } }
  ]
}
```

Allowed — carries `change-approved`:

```json
{
  "environment": "prod",
  "actions": [
    { "kind": "k8s.apply", "risk_tags": ["change-approved"], "payload": { "namespace": "prod" } }
  ]
}
```

---

### ops.public_exposure

**Intent**

Enforces that Terraform plans which expose resources publicly carry the `approved_public` risk tag. This rule prevents unguarded creation of internet-accessible infrastructure.

**Applies to**

Actions of kind `terraform.plan` where `payload.publicly_exposed` is `true`.

**Trigger**

Fires when at least one `terraform.plan` action has `payload.publicly_exposed == true` and does not carry the `approved_public` risk tag.

**Outputs**

- Adds `"ops.public_exposure"` to `hits`.
- Adds `"Public exposure requires approved_public"` to `reasons`.
- Sets `allow` to `false`.
- Includes hints for `ops.public_exposure` in the `hints` array.

**Params**

This rule uses no configurable params. The `publicly_exposed` field is a direct payload attribute.

| Param Key | Type | Default | Environment-aware |
|---|---|---|---|
| _(none)_ | — | — | no |

**Hints**

- `Add risk_tag: approved_public`
- `Or remove public exposure`

**Examples**

Denied — public exposure, no approval tag:

```json
{
  "environment": "dev",
  "actions": [
    { "kind": "terraform.plan", "risk_tags": [], "payload": { "publicly_exposed": true } }
  ]
}
```

Allowed — carries `approved_public`:

```json
{
  "environment": "dev",
  "actions": [
    { "kind": "terraform.plan", "risk_tags": ["approved_public"], "payload": { "publicly_exposed": true } }
  ]
}
```

---

### ops.autonomous_execution

**Intent**

Flags requests that originate from an autonomous agent via the MCP interface. The rule does not deny; it emits a warning to inform reviewers that the operation was not initiated by a human and warrants manual review.

**Applies to**

Any request where the actor type is `"agent"` and the source is `"mcp"`. Individual action kinds are not inspected.

**Trigger**

Fires when the request actor type is `"agent"` and the request source is `"mcp"`.

**Outputs**

- Adds `"ops.autonomous_execution"` to `hits` (via warn; does not deny).
- Does not affect `allow` or `reasons`.
- Includes hints for `ops.autonomous_execution` in the `hints` array.

**Params**

This rule uses no configurable params.

| Param Key | Type | Default | Environment-aware |
|---|---|---|---|
| _(none)_ | — | — | no |

**Hints**

- `Review changes manually before apply`

**Examples**

Warn fired — agent request via MCP:

```json
{
  "source": "mcp",
  "actor": { "type": "agent" },
  "environment": "dev",
  "actions": [
    { "kind": "kubectl.apply", "risk_tags": [], "payload": { "namespace": "default" } }
  ]
}
```

Decision: `allow: true`, `hits: ["ops.autonomous_execution"]`, `hints: ["Review changes manually before apply"]`.

No warn — human request:

```json
{
  "source": "cli",
  "actor": { "type": "human" },
  "environment": "dev",
  "actions": [
    { "kind": "kubectl.apply", "risk_tags": [], "payload": { "namespace": "default" } }
  ]
}
```

---

### ops.breakglass_used

**Intent**

Emits a warning whenever any action carries the `breakglass` risk tag. This ensures that breakglass usage is always recorded in the decision output for audit, even when the tag legitimately bypasses a deny rule.

**Applies to**

Any action carrying `"breakglass"` in its `risk_tags` list, regardless of kind.

**Trigger**

Fires when at least one action in the request has `"breakglass"` in its `risk_tags` array.

**Outputs**

- Adds `"ops.breakglass_used"` to `hits` (via warn; does not deny).
- Does not affect `allow` or `reasons`.
- Includes hints for `ops.breakglass_used` in the `hints` array.

**Params**

This rule uses no configurable params.

| Param Key | Type | Default | Environment-aware |
|---|---|---|---|
| _(none)_ | — | — | no |

**Hints**

- `Use breakglass only for emergencies.`
- `Prefer change-approved tags for non-emergency production changes.`

**Examples**

Warn fired — breakglass tag present (bypasses `k8s.protected_namespace` deny):

```json
{
  "environment": "prod",
  "source": "cli",
  "actor": { "type": "human" },
  "actions": [
    { "kind": "kubectl.apply", "risk_tags": ["breakglass"], "payload": { "namespace": "kube-system" } }
  ]
}
```

Decision: `allow: true`, `hits: ["ops.breakglass_used"]`.

No warn — no breakglass tag:

```json
{
  "environment": "dev",
  "actions": [
    { "kind": "kubectl.apply", "risk_tags": [], "payload": { "namespace": "default" } }
  ]
}
```

---

## 4. Params Index

| Param Key | Used By Rules | Type | Has `by_env.default` | Description |
|---|---|---|---|---|
| `ops.mass_delete.max_deletes` | `ops.mass_delete` | number | yes (`5`) | Maximum resource count per delete/destroy operation before denial. Overridden to `3` in `production`. |
| `k8s.namespaces.restricted` | `k8s.protected_namespace` | list of strings | yes (`["kube-system"]`) | Namespaces requiring a `breakglass` tag for any action. |
| `k8s.namespaces.protected` | `ops.unapproved_change` | list of strings | yes (`["prod"]`) | Namespaces requiring a `change-approved` tag for any action. |

All three params define a `by_env.default` and are functional without specifying an environment. No params are currently defined but unused.

---

## 5. Authoring Guidelines

The following conventions apply to all policy contributions in this bundle. For normative MUST/MUST NOT language and the full rule authoring contract, see [POLICY_TECHNICAL_SPEC.md §7](POLICY_TECHNICAL_SPEC.md).

**Rule IDs**

Format: `domain.invariant_name` — lowercase, dot-separated, exactly two segments. No environment names, severity labels, or ordinal suffixes.

Valid: `ops.mass_delete`, `k8s.protected_namespace`.
Invalid: `ops.mass_delete.prod`, `ops.high_mass_delete`, `ops.mass_delete_v2`.

**Tunable values**

All configurable thresholds and lists belong in `evidra/data/params/data.json`, not in rule bodies. Every param must define `by_env.default`.

**Environment in rules**

Rules must not reference environment names as string literals. Environment-specific behavior is achieved solely through `resolve_param` / `resolve_list_param` with `by_env` entries in `data.json`.

**Hints**

Hint strings belong in `evidra/data/rule_hints/data.json`, keyed by the same canonical rule ID used in the rule head. Aim for 1–3 actionable strings per rule.

---

## 6. Coverage Notes

### Rules lacking hints

No active rules are missing hints. All six rule IDs have entries in `rule_hints/data.json`.

**Note:** `sys.unlabeled_deny` has a hint entry in `rule_hints/data.json` but has no corresponding Rego rule file. It is a reserved rule ID for a meta-enforcement convention (ensuring deny rules emit a label). It is not emitted by any current rule.

### Params lacking `by_env.default`

None. All three params define a `by_env.default`.

### Rules where scope could not be inferred from logic alone

- **`ops.autonomous_execution`** — inspects top-level request fields (`actor.type`, `source`), not individual action kinds. Applies to any request arriving from an agent via MCP. A doc comment in the rule file would make this explicit.
- **`ops.breakglass_used`** — matches any action with `breakglass` in `risk_tags`, independent of kind. Scope is implicit from the tag check; a doc comment would clarify intent.
