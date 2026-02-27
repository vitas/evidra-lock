# Evidra — Kill-Switch Execution Plan v0.6

This document defines the minimal changes to turn Evidra into a reliable
AI kill-switch. No architectural rewrite. No new schema fields. No execution
responsibility. No adapters required for MCP flow.

Goal: destructive operations with incomplete, hallucinated, or structurally
nonsensical payloads are denied by default. Safe operations pass without friction.

---

## Data Flow Reference

Understanding the data flow is critical to writing correct rules.

```
MCP agent calls validate tool with:
  tool: "kubectl"
  operation: "delete"
  params: { target: {namespace: "kube-system"}, payload: {resource: "pod"} }

→ invocationToScenario() builds:
  Action{ Kind: "kubectl.delete", Target: {namespace:...}, Payload: {resource:...} }

→ evaluateScenarioWithRuntime() wraps into:
  Params["action"] = { kind, target, payload, risk_tags }

→ Engine.Evaluate() calls buildActionList() → OPA receives:
  input.actions[0].kind = "kubectl.delete"
  input.actions[0].payload.namespace = "kube-system"  (via action_namespace helper)
  input.actions[0].payload.resource = "pod"
  input.actions[0].risk_tags = []
```

All existing rules operate on `input.actions[_]`. New rules must follow the
same pattern.

---

## Evaluation Order

OPA evaluates all rules simultaneously — there is no procedural ordering.
However, the guards form a logical precedence for understanding behavior:

```
1. Unknown tool guard       → is this tool/operation recognized at all?
2. Insufficient context     → does the payload have enough for evaluation?
3. Truncation guard         → is the data complete or cut short?
4. Domain-specific policies → k8s, terraform, argocd, iam, s3 rules
```

If multiple rules fire, all hits are returned in the response. The agent
sees the full list of violations, not just the first one.

For debugging: if a deny comes back with `ops.unknown_destructive`, the
tool is not in any known list — check params first. If `ops.insufficient_context`,
the payload is missing fields — check the required context table below.

---

## Threat Model

AI agents interacting with infrastructure via MCP may:

- Omit required fields (hallucinated or lazy payload construction)
- Provide partial structures (containers without security_context)
- Send empty objects that pass existence checks (containers: [{}])
- Mislabel operations (destructive action described as non-destructive)
- Invoke unknown tools not covered by existing policy
- Send fabricated counts to bypass threshold rules (destroy_count: 0)

The kill-switch design assumes:

- Payload may be incomplete, incorrect, or adversarial
- Fail-closed is safer than silent allow
- Read-only operations should never be blocked (agent must keep calling validate)
- Every deny must include actionable hints (agent must know how to fix it)
- Breakglass escape hatch must exist (users will remove Evidra if they can't override)

---

## Sufficient Context Definition

Sufficient context means: **the minimal set of fields required for at least
one applicable policy rule to evaluate the operation meaningfully.**

If no applicable rule can evaluate the payload — because required fields are
missing, empty, or structurally invalid — the operation is denied by the
fail-closed guard. This is not a policy violation; it is an inability to
determine safety.

The required fields per operation are defined in the table below and
encoded in `has_sufficient_context()` rego rules. They are derived directly
from what existing policy rules read from `input.actions[_].payload`.

---

## Step 1 — Fail-Closed Guard Rule

### Objective

Deny destructive operations that arrive with empty or insufficient payload.
This is the single highest-impact change in the plan.

### Destructive Operations (explicit set)

```
destructive_operations := {
    "kubectl.delete",
    "kubectl.apply",
    "terraform.apply",
    "terraform.destroy",
    "helm.upgrade",
    "helm.uninstall",
    "argocd.sync",
}
```

Stored in `evidra/data/params/data.json` as a list param, not hardcoded in rule body.
Extensible without policy code changes.

### Non-Destructive Operations (safe suffix for known tools)

There is no explicit list of non-destructive operations. Instead,
read-only operations are identified by suffix for known tool prefixes:

```
known_tool_prefixes := {"kubectl", "terraform", "helm", "argocd"}
safe_suffixes := {"get", "list", "describe", "show", "diff", "plan", "status", "version"}
```

Any `known_prefix.safe_suffix` (e.g. `kubectl.get`, `terraform.plan`)
passes without payload check. Unknown tool prefixes never get safe suffix
treatment — see Step 4.

This eliminates a separate params list to maintain. If a known tool has a
non-standard read-only operation (e.g. `kubectl.logs`), add the suffix to
`safe_suffixes` — one place, not two lists.

### Required Context Per Operation

The guard fires when a destructive operation lacks the minimum fields needed
for any policy rule to meaningfully evaluate it.

| operation | required in payload | rationale |
|---|---|---|
| `kubectl.delete` | `namespace` | k8s.protected_namespace needs it; mass_delete needs `resource_count` or namespace |
| `kubectl.apply` | `namespace`; if resource is workload (pod/deployment/statefulset/daemonset/replicaset/job/cronjob) also requires `containers[]` | k8s.protected_namespace needs namespace; container rules need containers for workload resources |
| `terraform.apply` | At least one semantic detail: `resource_types` (non-empty), or plausible `security_group_rules`, `iam_policy_statements`, `trust_policy_statements`, or non-empty `s3_public_access_block`, `server_side_encryption` | Detail ensures domain rules have something to evaluate. Counts (`destroy_count`, `total_changes`) are optional — mass_delete checks them when present |
| `terraform.destroy` | `destroy_count` | ops.mass_delete threshold check needs it |
| `helm.upgrade` | `namespace` | namespace protection |
| `helm.uninstall` | `namespace` | namespace protection |
| `argocd.sync` | `app_name` or `sync_policy` | argocd rules need sync config |

### Rule Implementation

File: `policy/bundles/ops-v0.1/evidra/policy/rules/deny_insufficient_context.rego`

```rego
package evidra.policy

import data.evidra.policy.defaults as defaults

# ──────────────────────────────────────────────────────────
# Fail-closed: destructive operation with insufficient payload.
#
# If kind is in destructive_operations and no has_sufficient_context
# branch matches → deny. This covers both "known tool, missing fields"
# and "user-added tool without a clause" — same rule, same hint.
# ──────────────────────────────────────────────────────────

deny["ops.insufficient_context"] = msg if {
    action := input.actions[_]
    is_destructive(action.kind)
    not has_sufficient_context(action)
    msg := sprintf(
        "Destructive operation %s lacks required context. If this is a new tool, add a has_sufficient_context clause.",
        [action.kind]
    )
}

is_destructive(kind) if {
    ops := defaults.resolve_list_param("ops.destructive_operations")
    ops[_] == kind
}

# ── kubectl ──────────────────────────────────────────────

has_sufficient_context(action) if {
    action.kind == "kubectl.delete"
    defaults.action_namespace(action) != ""
}

has_sufficient_context(action) if {
    action.kind == "kubectl.apply"
    defaults.action_namespace(action) != ""
    payload := object.get(action, "payload", {})
    not is_workload_resource(payload)
}

has_sufficient_context(action) if {
    action.kind == "kubectl.apply"
    defaults.action_namespace(action) != ""
    payload := object.get(action, "payload", {})
    is_workload_resource(payload)
    has_real_containers(payload)
}

has_real_containers(payload) if {
    some c in defaults.all_containers(payload)
    object.get(c, "image", "") != ""
}

workload_kinds := {"pod", "deployment", "statefulset", "daemonset", "replicaset", "job", "cronjob"}

is_workload_resource(payload) if {
    raw := object.get(payload, "resource", object.get(payload, "kind", ""))
    resource := trim_space(lower(raw))
    workload_kinds[resource]
}

# ── terraform ────────────────────────────────────────────

has_sufficient_context(action) if {
    action.kind == "terraform.apply"
    payload := object.get(action, "payload", {})
    terraform_has_detail(payload)
}

has_sufficient_context(action) if {
    action.kind == "terraform.destroy"
    payload := object.get(action, "payload", {})
    is_number(object.get(payload, "destroy_count", null))
}

# Detail = at least one structurally plausible semantic signal.
# Counts alone (destroy_count, total_changes) do NOT satisfy this.

terraform_has_detail(payload) if {
    count(object.get(payload, "resource_types", [])) > 0
}

terraform_has_detail(payload) if {
    has_nonempty_object(object.get(payload, "s3_public_access_block", null))
}

terraform_has_detail(payload) if {
    has_nonempty_object(object.get(payload, "server_side_encryption", null))
}

terraform_has_detail(payload) if {
    has_plausible_sg_rules(object.get(payload, "security_group_rules", []))
}

terraform_has_detail(payload) if {
    has_plausible_statements(object.get(payload, "iam_policy_statements", []))
}

terraform_has_detail(payload) if {
    has_plausible_statements(object.get(payload, "trust_policy_statements", []))
}

has_nonempty_object(x) if {
    x != null
    is_object(x)
    count(x) > 0
}

has_plausible_sg_rules(rules) if {
    count(rules) > 0
    some r in rules
    is_object(r)
    plausible_sg_keys := {"cidr", "cidr_blocks", "from_port", "to_port", "protocol", "direction", "type"}
    some k in plausible_sg_keys
    r[k] != null
}

has_plausible_statements(stmts) if {
    count(stmts) > 0
    some s in stmts
    is_object(s)
    plausible_stmt_keys := {"Action", "NotAction", "Resource", "NotResource", "Effect", "Principal", "Condition"}
    some k in plausible_stmt_keys
    s[k] != null
}

# ── helm ─────────────────────────────────────────────────

has_sufficient_context(action) if {
    action.kind == "helm.upgrade"
    defaults.action_namespace(action) != ""
}

has_sufficient_context(action) if {
    action.kind == "helm.uninstall"
    defaults.action_namespace(action) != ""
}

# ── argocd ───────────────────────────────────────────────

has_sufficient_context(action) if {
    action.kind == "argocd.sync"
    payload := object.get(action, "payload", {})
    has_argocd_context(payload)
}

has_argocd_context(payload) if { payload.app_name != "" }
has_argocd_context(payload) if { _ = payload.sync_policy }
```

### Hints

Add to `evidra/data/rule_hints/data.json`:

```json
"ops.insufficient_context": [
    "Provide required context in payload for this operation.",
    "For kubectl: include namespace. For workload resources (pod/deployment/statefulset/daemonset/job/cronjob): also include containers[] with image.",
    "For terraform: include at least one semantic detail (resource_types, security_group_rules, iam_policy_statements, etc.).",
    "If this is a newly added tool: add a has_sufficient_context clause in deny_insufficient_context.rego."
]
```

### Params

Add to `evidra/data/params/data.json`:

```json
"ops.destructive_operations": {
    "by_env": {
        "default": [
            "kubectl.delete", "kubectl.apply",
            "terraform.apply", "terraform.destroy",
            "helm.upgrade", "helm.uninstall",
            "argocd.sync"
        ]
    }
}
```

---

## Note: Intent Derivation

No new schema fields. Intent is derived inside policy from `action.kind`.
The destructive set from Step 1 is the single source of truth.
`is_destructive(kind)` is reused anywhere intent matters.
No string matching (no "contains delete"). Explicit set only.
This is a design decision, not an implementation step.

---

## Step 2 — MCP Tool Description Rewrite

### Current (too vague)

```
"Run the validation scenario without executing commands. Provide tool/operation
metadata and risk tags to inspect policy hits, hints, and evidence."
```

### New

```
"REQUIRED before executing any infrastructure command that creates, modifies,
or deletes resources. Call this BEFORE running: kubectl apply/delete,
terraform apply/destroy, helm upgrade/uninstall, argocd sync.

Provide in params: tool name, operation, and context in payload —
at minimum the target namespace (for k8s/helm), container specs for
workload resources (pod/deployment/statefulset/daemonset), or resource
counts and plan details (for terraform).

If the result shows allow=false: DO NOT proceed with the operation.
Show the denial reasons and hints to the user and wait for guidance.
Never retry a denied operation without changing parameters.

Safe to skip for read-only operations: plan, get, describe, list, show, diff."
```

### Why each line matters

Line 1 — tells agent WHEN to call (any mutating operation).
Line 2 — explicit list so agent doesn't guess.
Line 3 — tells agent WHAT to put in payload (minimum viable context).
Line 4-5 — tells agent what to do on deny (stop, show, wait).
Line 6 — tells agent when NOT to call (prevents UX friction on reads).

---

## Step 3 — End-to-End Tests

Tests simulate realistic MCP-shaped input (what Claude Code would actually send)
going through the full validate pipeline.

### Test matrix

| # | test name | input | expected | validates |
|---|---|---|---|---|
| 1 | `e2e_readonly_no_payload_allow` | tool=kubectl, op=get, payload={} | ALLOW | Read-only ops pass without payload |
| 2 | `e2e_terraform_plan_no_payload_allow` | tool=terraform, op=plan, payload={} | ALLOW | plan is non-destructive |
| 3 | `e2e_destructive_empty_payload_deny` | tool=terraform, op=apply, payload={} | DENY (ops.insufficient_context) | Fail-closed works |
| 4 | `e2e_kubectl_delete_no_namespace_deny` | tool=kubectl, op=delete, payload={resource:"pod"} | DENY (ops.insufficient_context) | Missing namespace caught |
| 5 | `e2e_kubectl_delete_protected_ns_deny` | tool=kubectl, op=delete, payload={namespace:"kube-system",resource:"pod"} | DENY (k8s.protected_namespace) | Protected namespace rule fires |
| 6 | `e2e_kubectl_delete_safe_ns_allow` | tool=kubectl, op=delete, payload={namespace:"default",resource:"pod"} | ALLOW | Valid delete in safe namespace passes |
| 7 | `e2e_terraform_public_s3_deny` | tool=terraform, op=apply, payload with s3_public_access_block missing | DENY (terraform.s3_public_access) | Terraform rule fires on MCP input |
| 8 | `e2e_terraform_iam_wildcard_deny` | tool=terraform, op=apply, payload with Action:* Resource:* | DENY (terraform.iam_wildcard_policy) | IAM rule fires |
| 9 | `e2e_privileged_container_deny` | tool=kubectl, op=apply, payload with containers[].security_context.privileged=true, namespace, resource="deployment" | DENY (k8s.privileged_container) | K8s container rule fires |
| 10a | `e2e_workload_apply_no_containers_deny` | tool=kubectl, op=apply, payload={namespace:"default", resource:"deployment"} (no containers) | DENY (ops.insufficient_context) | Workload without containers = insufficient context |
| 10b | `e2e_nonworkload_apply_no_containers_allow` | tool=kubectl, op=apply, payload={namespace:"default", resource:"configmap"} | ALLOW | Non-workload resource does not require containers |
| 11 | `e2e_unknown_tool_destructive_deny` | tool=pulumi, op=up, payload={} | DENY (ops.unknown_destructive) | Unknown tool guard (Step 6) |
| 12 | `e2e_valid_terraform_apply_allow` | tool=terraform, op=apply, payload={destroy_count:0, total_changes:1, resource_types:["aws_instance"]} | ALLOW | Valid apply passes (counts + detail) |
| 13 | `e2e_terraform_counts_only_deny` | tool=terraform, op=apply, payload={destroy_count:0} | DENY (ops.insufficient_context) | Counts alone are not semantic detail — denied |
| 14 | `e2e_unknown_tool_safe_suffix_deny` | tool=pulumi, op=plan, payload={} | DENY (ops.unknown_destructive) | Safe suffix does NOT apply to unknown tools |
| 15 | `e2e_unknown_tool_breakglass_allow` | tool=crossplane, op=apply, payload={}, risk_tags=["breakglass"] | ALLOW (with ops.breakglass_used warn) | Breakglass overrides unknown tool guard |
| 16 | `e2e_workload_fake_empty_containers_deny` | tool=kubectl, op=apply, payload={namespace:"default", resource:"deployment", containers:[{}]} | DENY (ops.insufficient_context) | Empty container objects (no image) = insufficient context |
| 17 | `e2e_terraform_detail_nonsense_deny` | tool=terraform, op=apply, payload={destroy_count:0, total_changes:1, security_group_rules:[{"foo":"bar"}]} | DENY (ops.insufficient_context) | "Shape-only" detail with no plausible keys does not count |
| 18 | `e2e_terraform_s3_block_empty_object_deny` | tool=terraform, op=apply, payload={destroy_count:0, total_changes:1, s3_public_access_block:{}} | DENY (ops.insufficient_context) | Empty object does not satisfy detail |
| 19 | `e2e_added_tool_no_clause_deny` | tool=crossplane, op=apply (in destructive_operations, no has_sufficient_context clause), payload={anything:true} | DENY (ops.insufficient_context) | Added tool without context clause — no branch matches → deny |
| 20 | `e2e_added_tool_read_op_auto_allow` | tool=crossplane, op=get (crossplane.apply in destructive_operations), payload={} | ALLOW | Derived known prefix + safe suffix → auto-allow read ops |

### Test format

Each test is a JSON file in `tests/e2e/` and a corresponding rego test or
Go test that:

1. Constructs a `ToolInvocation` matching MCP input shape
2. Runs through `validate.EvaluateInvocation()`
3. Asserts `Pass` true/false and expected `RuleIDs`

### Positive tests matter

Tests 1, 2, 6, 10, 12 are ALLOW tests. Without them we don't know
the allow path works. A kill-switch that blocks everything is not a
kill-switch — it's an off-switch. Agents will stop calling validate.

---

## Step 5 — Terraform Truncation Guard

**Scope: adapter flow only.** This step applies when the Terraform adapter
produces structured output. In the MCP-only flow (LLM fills payload), the
agent won't send truncation flags — Step 1 handles that case.

### Rule

```rego
deny["ops.truncated_context"] = "Plan output is truncated — manual review required" if {
    action := input.actions[_]
    is_destructive(action.kind)
    payload := object.get(action, "payload", {})
    is_truncated(payload)
}

is_truncated(payload) if { payload.resource_changes_truncated == true }
is_truncated(payload) if { payload.delete_addresses_truncated == true }
is_truncated(payload) if { payload.replace_addresses_truncated == true }
```

### Hints

```json
"ops.truncated_context": [
    "Plan output was truncated and may be incomplete.",
    "Review the full plan manually before applying.",
    "Or reduce plan scope with -target flags."
]
```

---

## Step 4 — Unknown Tool Guard

### Problem

If an agent calls validate with `tool: "pulumi"`, `operation: "up"` — no
existing rule matches. Result: allow. This is a silent bypass.

### Rule

```rego
deny["ops.unknown_destructive"] = msg if {
    action := input.actions[_]
    not is_known_operation(action.kind)
    not is_safe_read_operation(action.kind)
    not defaults.has_tag(action, "breakglass")
    msg := sprintf("Unknown operation %s — cannot evaluate safety", [action.kind])
}

is_known_operation(kind) if {
    ops := defaults.resolve_list_param("ops.destructive_operations")
    ops[_] == kind
}

# Read-only operations: known tool prefix + safe suffix.
# known_tool_prefixes is DERIVED from destructive_operations — no separate list.
# When user adds "crossplane.apply" to destructive_operations, "crossplane"
# automatically becomes a known prefix and "crossplane.get" passes.

known_tool_prefixes[prefix] if {
    ops := defaults.resolve_list_param("ops.destructive_operations")
    kind := ops[_]
    parts := split(kind, ".")
    count(parts) == 2
    prefix := parts[0]
}

safe_suffixes := {"get", "list", "describe", "show", "diff", "plan", "status", "version"}

is_safe_read_operation(kind) if {
    parts := split(kind, ".")
    count(parts) == 2
    known_tool_prefixes[parts[0]]
    safe_suffixes[parts[1]]
}
```

### Breakglass override

The unknown tool guard respects `breakglass` risk tag. This allows emergency
use of unrecognized tools without removing Evidra from the pipeline.
The existing `warn["ops.breakglass_used"]` rule ensures every breakglass
usage is logged to the evidence chain.

**Breakglass scope is limited:**

| rule | breakglass bypasses? | rationale |
|---|---|---|
| `ops.unknown_destructive` | YES | Emergency use of unrecognized tools |
| `ops.insufficient_context` | NO | Missing payload is never safe, even in emergencies |
| `ops.truncated_context` | NO | Incomplete data is never safe |
| Domain rules (k8s.*, terraform.*, etc.) | NO | These catch real violations, not missing context |

Breakglass is an escape hatch for unknown tools, not a superuser mode.
It does NOT bypass insufficient_context — a destructive operation added
to the list without a `has_sufficient_context` clause will be denied by
`ops.insufficient_context` even with breakglass, because no clause matches.

Example: agent needs to run `crossplane.apply` in an emergency:
```json
{ "tool": "crossplane", "operation": "apply", "params": { "risk_tags": ["breakglass"] } }
```
→ ALLOW (with breakglass warning in evidence)

### Hints

```json
"ops.unknown_destructive": [
    "This tool/operation is not recognized by Evidra policy.",
    "Add it to ops.destructive_operations in policy params, or use a known safe suffix (get, list, describe, plan, etc.).",
    "For emergency use: add risk_tag: breakglass to override (will be logged)."
]
```

### Safety net

`is_safe_read_operation` allows read-only operations ONLY for tool
prefixes derived from `ops.destructive_operations`. This means:

- `kubectl.get` → ALLOW (kubectl is in destructive_operations → known prefix)
- `terraform.plan` → ALLOW (terraform is known → safe suffix)
- `pulumi.plan` → DENY (pulumi not in any list → unknown prefix)
- `crossplane.get` → if crossplane.apply is in destructive_operations → ALLOW (auto-derived)
- `crossplane.get` → if crossplane not in any list → DENY

When a user adds `crossplane.apply` to `ops.destructive_operations`,
`crossplane` automatically becomes a known prefix. No second list to update.

---

## Note: Deny Response Quality

Every deny response must tell the agent exactly why and what to do.
This is not a separate implementation step — it is built into Steps 1, 3, and 4.

Response structure (already exists):

```json
{
    "allow": false,
    "risk_level": "high",
    "hits": ["ops.insufficient_context"],
    "reasons": ["Destructive operation kubectl.delete requires payload context"],
    "hints": [
        "Provide required context in payload for this operation.",
        "For kubectl: include namespace."
    ]
}
```

Requirements for all new rules:

1. A `reason` string that names the operation (via sprintf)
2. 2-3 `hints` in rule_hints/data.json that tell the agent what to provide
3. Hint language written for LLM consumption — concrete, actionable, no jargon

Example bad hint: "Ensure sufficient input context is provided."
Example good hint: "For kubectl: include namespace. For terraform: include resource counts or plan details."

---

## File Mapping

What to create and where:

| file | what | steps |
|---|---|---|
| `evidra/policy/rules/deny_insufficient_context.rego` | Fail-closed guard + all helpers (`is_destructive`, `has_sufficient_context`, `terraform_has_detail`, `workload_kinds`, plausibility checks) | 1 |
| `evidra/policy/rules/deny_unknown_destructive.rego` | Unknown tool guard + `is_known_operation`, `known_tool_prefixes` (derived), `is_safe_read_operation` | 4 |
| `evidra/policy/rules/deny_truncated_context.rego` | Truncation guard (adapter flow) | 3 |
| `evidra/data/params/data.json` | Add `ops.destructive_operations`, `ops.profile` | 1, 4 |
| `evidra/data/rule_hints/data.json` | Add hints for `ops.insufficient_context`, `ops.unknown_destructive`, `ops.truncated_context` | 1, 3, 4 |
| `pkg/mcpserver/server.go` | Rewrite tool description (line ~118) | 2 |
| `tests/e2e/*.json` or `*_test.go` | 20 E2E test fixtures + assertions | 3 |

All rule files go in `policy/bundles/ops-v0.1/evidra/policy/rules/`.

---

## Implementation Order

| priority | what | effort | impact |
|---|---|---|---|
| P0 | Fail-closed guard rule (Step 1) | 1 day | Closes 80% of kill-switch gaps |
| P0 | MCP tool description rewrite (Step 2) | 1 hour | Changes agent behavior immediately |
| P0 | E2E tests — first 8 (Step 3) | 1 day | Validates fail-closed + unknown tool work |
| P0 | Terraform detail plausibility | 2 hours | Blocks nonsense detail bypass |
| P1 | Unknown tool guard (Step 4) | 2 hours | Closes silent bypass on unknown tools |
| P1 | E2E tests — remaining 12 (Step 3) | 0.5 day | Full coverage |
| P1 | Breakglass scope documentation | 30 min | Prevents misconceptions |
| P2 | Truncation guard (Step 5) | 1 hour | Adapter flow only |

Total: ~3 days of work.

---

## Adding New Tools (Extensibility Guide)

When a user adopts a new infrastructure tool (e.g. Crossplane, Pulumi, CDK),
the unknown tool guard will deny all operations. This is intentional — unknown
tools cannot be evaluated for safety.

To add support for a new tool, edit `evidra/data/params/data.json`:

```json
"ops.destructive_operations": {
    "by_env": {
        "default": [...existing..., "crossplane.apply", "crossplane.delete"]
    }
}
```

This does two things automatically:
1. `crossplane` becomes a known tool prefix → `crossplane.get`, `crossplane.list` etc. pass via safe suffix
2. `crossplane.apply` and `crossplane.delete` are recognized as destructive → require `has_sufficient_context`

**There is no separate "specifically checked" or "supported tools" list.**
A kind is considered supported if and only if it has a `has_sufficient_context`
clause. Adding a kind to `ops.destructive_operations` without a clause
results in `ops.insufficient_context` deny — the same rule, the same
mechanism as kubectl with missing namespace. One list, one mechanism.

**You must also add a `has_sufficient_context` clause** in
`deny_insufficient_context.rego`. Without it, `ops.insufficient_context`
fires because no branch matches — this is the same rule that fires for
kubectl with missing namespace. Same mechanism, same hint.

Example minimal clause for a new tool:

```rego
has_sufficient_context(action) if {
    action.kind == "crossplane.apply"
    payload := object.get(action, "payload", {})
    count(payload) > 0    # accept any non-empty payload
}
```

For emergency use before adding rules: use `risk_tag: breakglass` to
bypass the unknown tool guard. Note: breakglass does NOT bypass
insufficient_context — if the tool is in destructive_operations without
a clause, it is denied even with breakglass.

---

## Payload Growth Governance

The biggest long-term risk is not missing rules — it's payload bloat.
Every new rule wants more fields, every field makes the MCP contract
heavier, and eventually the LLM can't reliably fill the schema.

These principles prevent that.

### Principle 1: Rule budget

Every new domain rule must declare what it reads. Before adding a rule,
answer:

| question | example |
|---|---|
| What payload paths does it read? | `action.payload.containers[_].security_context.privileged` |
| What is the minimum input for it to fire? | `containers` with `security_context` |
| Does this require a new gating field in Step 1? | No — containers are already gated for workloads |

If a rule requires a new gating field, it must update the required
context table. If it requires a field that LLMs cannot reliably produce,
it belongs in the adapter flow, not the MCP flow.

Process: add a `reads:` comment header to each rule file:

```rego
# reads: action.payload.containers[_].security_context.privileged
# min_context: containers[] with security_context
# gating: existing (workload + real containers)
deny["k8s.privileged_container"] = msg if { ... }
```

### Principle 2: Gating vs Semantic fields

Two classes of payload fields, with different ownership:

**Gating fields** (Step 1 / fail-closed guard):
- namespace, resource/kind, counts (destroy_count, total_changes)
- Presence of containers (with image)
- These are cheap, LLM can always fill them, low hallucination risk

**Semantic fields** (domain rules / golden 23):
- security_context.privileged, capabilities.add
- security_group_rules[].cidr_blocks, from_port, to_port
- iam_policy_statements[].Action, Resource
- s3_public_access_block settings

Rule: **Step 1 depends only on gating fields.** Semantic fields are
checked by domain rules which only fire if gating context is present.
This keeps the kill-switch layer thin and stable.

If a semantic field becomes so important that its absence should deny,
the correct fix is a new gating check — not expanding Step 1 to parse
semantic content.

### Principle 3: Don't add fields "for the future"

The required context table must reflect only what **existing rules
actually read**. No speculative fields. If no rule checks
`readinessProbe`, don't add it to the workload gating check.

When a new rule is added that needs a new field, the field is added
then — not before.

### Principle 4: Rich input comes from adapters, not LLMs

For Terraform: if users want deeper validation (module dependencies,
provider config, state drift), the answer is the Terraform adapter
pipeline — not a bigger MCP payload. The MCP payload should stay small
enough that an LLM fills it correctly >95% of the time.

The boundary:

| source | payload size | when |
|---|---|---|
| LLM via MCP | 5-15 fields | Real-time agent workflow |
| Adapter via CLI/CI | 50+ fields | Pipeline, pre-apply |

If someone asks "can Evidra check X via MCP?" and X requires parsing
a 10MB terraform plan — the answer is "use the adapter", not "add more
fields to validate tool".

---

## Known Limitations

These are conscious design boundaries, not bugs. Documenting them
prevents false expectations.

**1. Non-workload kubectl.apply is not content-validated.**
`kubectl.apply` with `resource: "configmap"` and valid namespace passes
the context guard. If the configmap contains dangerous data (e.g. overrides
to critical service config), Evidra will not catch it. Evidra is a
catastrophic-action kill-switch, not a full configuration validator.

**2. User-added destructive tools require a context clause.**
A tool added to `ops.destructive_operations` without a corresponding
`has_sufficient_context` clause is denied by `ops.insufficient_context`
(no branch matches → deny). This is the same mechanism as kubectl with
missing namespace — not a separate rule. Add a clause to define what
"sufficient" means for the new tool.

**3. Nested kind formats are not supported.**
`has_sufficient_context` branches match on exact `action.kind` strings
(e.g. `"helm.upgrade"`). A hypothetical `kubectl.batch.delete` would not
match any branch and would be denied by `ops.insufficient_context`.
Current convention is `tool.operation` (one dot). If nested kinds become
necessary, the matching logic must be updated.

**4. terraform.destroy requires only destroy_count.**
Unlike `terraform.apply` which requires counts + semantic detail,
`terraform.destroy` requires only `destroy_count`. This is correct —
destroy operations don't involve SG/IAM/S3 configuration checks.
The `ops.mass_delete` threshold rule evaluates the count.

---

## Non-Goals (Phase 2)

Not included in v0.6:

- Adapter registry
- Kubernetes adapter (kill-switch works without it via fail-closed)
- Execution wrapper (evidra.exec — Evidra validates, never executes)
- Admission controller
- Signed tokens
- SaaS dashboard
- `check_command` raw string input (LLM fills structured input via MCP)

These will only be considered if user demand appears after launch.

### Adapter Compatibility (Phase 2 design note)

The kill-switch architecture is designed to be adapter-compatible without
changes. When adapters are added, they work as preprocessors — not as a
separate flow:

```
Without adapter:  LLM fills payload → validate → OPA
With adapter:     adapter fills payload → validate → OPA
```

OPA sees the same input shape regardless of source. Adapter selection is
by operation, not by magic:

- `terraform.apply` + `plan_json` in payload → terraform adapter parses
  plan JSON, fills `resource_types`, `security_group_rules`, etc.
- `kubectl.apply` + `manifest_yaml` in payload → k8s adapter parses
  manifest, fills `containers`, `security_context`, etc.

No adapter registry needed. No new architecture. The adapter is a function
`func adapt(kind string, payload map[string]interface{}) map[string]interface{}`
that runs before `validate`. If the adapted payload still lacks sufficient
context (e.g. plan JSON was truncated), `ops.insufficient_context` or
`ops.truncated_context` fires as normal.

This is documented here to ensure kill-switch design does not close the
door to adapters. Implementation is Phase 2.

---

## Philosophy

Minimal change. Maximum safety. Fail closed on destructive operations.
Fail open on read-only operations. No architectural rewrite. No new
schema fields. No execution responsibility. Agent friction only where
it prevents catastrophe.

---

## Success Criteria

After implementation:

1. `terraform.apply` with empty payload → DENY
2. `terraform.apply` with `{"destroy_count": 0}` only → DENY (no semantic detail)
3. `terraform.apply` with `security_group_rules: [{"foo":"bar"}]` → DENY (nonsense detail)
4. `kubectl.delete` in kube-system → DENY
5. `kubectl.apply` with resource=deployment but no containers → DENY
6. `kubectl.apply` with resource=configmap, namespace only → ALLOW
7. `kubectl.get` with no payload → ALLOW (safe suffix)
8. `pulumi.up` → DENY (unknown tool)
9. `pulumi.plan` → DENY (unknown prefix, safe suffix does not apply)
10. `crossplane.apply` (not in any list) + breakglass → ALLOW
11. `crossplane.apply` added to destructive_operations without clause → DENY (ops.insufficient_context)
12. `crossplane.get` after crossplane.apply added → ALLOW (auto-derived known prefix)
13. Agent receives actionable hints on every deny
14. All 21 E2E tests pass

---

END OF PLAN v0.6
