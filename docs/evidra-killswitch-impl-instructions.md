# Evidra Kill-Switch — Implementation Instructions

These are step-by-step instructions for implementing the kill-switch
from `evidra-killswitch-plan-v0.6.md`. Each task is self-contained and
testable. Execute in order.

Reference plan: `evidra-killswitch-plan-v0.6.md` (read it first).

---

## Project context

- Go project at project root
- OPA policy rules: `policy/bundles/ops-v0.1/evidra/policy/rules/`
- OPA helpers: `policy/bundles/ops-v0.1/evidra/policy/defaults.rego`
- Policy params: `policy/bundles/ops-v0.1/evidra/data/params/data.json`
- Rule hints: `policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json`
- Decision aggregator: `policy/bundles/ops-v0.1/evidra/policy/decision.rego`
- MCP server: `pkg/mcpserver/server.go`
- Validate pipeline: `pkg/validate/validate.go`
- Existing tests: `pkg/validate/validate_test.go`
- OPA input shape: `input.actions[_].kind`, `input.actions[_].payload`, etc.
- Existing helpers: `defaults.action_namespace()`, `defaults.all_containers()`,
  `defaults.resolve_list_param()`, `defaults.resolve_param()`, `defaults.has_tag()`

---

## Task 1: Add params and hints

**Files to edit:**
- `policy/bundles/ops-v0.1/evidra/data/params/data.json`
- `policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json`

**What to do:**

Add to `params/data.json` (merge into existing JSON, don't overwrite):

```json
"ops.destructive_operations": {
    "by_env": {
        "default": [
            "kubectl.delete",
            "kubectl.apply",
            "terraform.apply",
            "terraform.destroy",
            "helm.upgrade",
            "helm.uninstall",
            "argocd.sync"
        ]
    }
},
"ops.profile": {
    "by_env": {
        "default": "ops"
    }
}
```

Add to `rule_hints/data.json` (merge, don't overwrite):

```json
"ops.insufficient_context": [
    "Provide required context in payload for this operation.",
    "For kubectl: include namespace. For workload resources (pod/deployment/statefulset/daemonset/job/cronjob): also include containers[] with image.",
    "For terraform: include at least one semantic detail (resource_types, security_group_rules, iam_policy_statements, etc.).",
    "If this is a newly added tool: add a has_sufficient_context clause in deny_insufficient_context.rego."
],
"ops.unknown_destructive": [
    "This tool/operation is not recognized by Evidra policy.",
    "Add it to ops.destructive_operations in policy params, or use a known safe suffix (get, list, describe, plan, etc.).",
    "For emergency use: add risk_tag: breakglass to override (will be logged)."
],
"ops.truncated_context": [
    "Plan output was truncated and may be incomplete.",
    "Review the full plan manually before applying.",
    "Or reduce plan scope with -target flags."
]
```

**Done when:** `go test ./pkg/policy/...` still passes (params/hints are data, no code change yet).

---

## Task 2: Create deny_insufficient_context.rego

**File to create:**
`policy/bundles/ops-v0.1/evidra/policy/rules/deny_insufficient_context.rego`

**Full content** — copy exactly from the `evidra-killswitch-plan-v0.6.md`
Step 1 Rule Implementation section. The file contains:

- `deny["ops.insufficient_context"]` rule
- `is_destructive(kind)` helper (reads from `ops.destructive_operations` param)
- `has_sufficient_context(action)` — one clause per kind:
  - `kubectl.delete` → namespace required
  - `kubectl.apply` → namespace + (non-workload OR workload with real containers)
  - `terraform.apply` → `terraform_has_detail(payload)`
  - `terraform.destroy` → `destroy_count` present
  - `helm.upgrade` → namespace
  - `helm.uninstall` → namespace
  - `argocd.sync` → `app_name` or `sync_policy`
- `is_workload_resource()`, `has_real_containers()`, `workload_kinds` set
- `terraform_has_detail()` with plausibility checks
- `has_nonempty_object()`, `has_plausible_sg_rules()`, `has_plausible_statements()`
- `has_argocd_context()`

**Important:** the file uses `package evidra.policy` and imports
`data.evidra.policy.defaults as defaults`, matching existing rules.

**Done when:** `go test ./pkg/policy/...` passes. Create a simple test
scenario: `terraform.apply` with empty payload → result must include
`ops.insufficient_context` in deny hits.

---

## Task 3: Create deny_unknown_destructive.rego

**File to create:**
`policy/bundles/ops-v0.1/evidra/policy/rules/deny_unknown_destructive.rego`

**Content** — from plan v0.6 Step 4 Rule section:

- `deny["ops.unknown_destructive"]` rule
- `is_known_operation(kind)` — checks `ops.destructive_operations` param
- `known_tool_prefixes[prefix]` — DERIVED from `ops.destructive_operations`
  (split on ".", take prefix). No hardcoded list.
- `safe_suffixes` set: `{"get", "list", "describe", "show", "diff", "plan", "status", "version"}`
- `is_safe_read_operation(kind)` — known prefix + safe suffix
- Breakglass: `not defaults.has_tag(action, "breakglass")`

**Key behavior:**
- `pulumi.up` → deny (unknown tool)
- `pulumi.plan` → deny (pulumi not a known prefix)
- `kubectl.get` → allow (kubectl is known prefix from destructive_operations, "get" is safe suffix)
- `crossplane.apply` + breakglass → allow
- Adding `crossplane.apply` to destructive_operations → `crossplane` auto-becomes known prefix

**Done when:** test `pulumi.up` → deny with `ops.unknown_destructive`.
Test `kubectl.get` with empty payload → allow (no deny rules fire).

---

## Task 4: Rewrite MCP tool description

**File to edit:** `pkg/mcpserver/server.go`, line ~118

**Replace** the current Description string:

```
"Run the validation scenario without executing commands. Provide tool/operation metadata and risk tags to inspect policy hits, hints, and evidence."
```

**With:**

```
"REQUIRED before executing any infrastructure command that creates, modifies, or deletes resources. Call this BEFORE running: kubectl apply/delete, terraform apply/destroy, helm upgrade/uninstall, argocd sync.\n\nProvide in params: tool name, operation, and context in payload — at minimum the target namespace (for k8s/helm), container specs for workload resources (pod/deployment/statefulset/daemonset), or resource counts and plan details (for terraform).\n\nIf the result shows allow=false: DO NOT proceed with the operation. Show the denial reasons and hints to the user and wait for guidance. Never retry a denied operation without changing parameters.\n\nSafe to skip for read-only operations: plan, get, describe, list, show, diff."
```

**Done when:** `go build ./...` passes. Tool description is visible when
MCP client lists tools.

---

## Task 5: E2E tests

**File to create:** `pkg/validate/killswitch_test.go`

Write tests using the same pattern as existing `validate_test.go`:
- Use `safeOpts(t)` for Options (points to `ops-v0.1` bundle)
- Build `scenario.Scenario` with specific `Action` configs
- Call `validate.EvaluateScenario()` 
- Assert `result.Pass` (true/false) and check `result.RuleIDs` for expected hits

**Test matrix** (from plan v0.6 — implement all 21):

```
# DENY tests — result.Pass must be false

1.  e2e_readonly_no_payload_allow
    kind="kubectl.get", payload={}
    → Pass=true (no deny rules fire, safe read op)

2.  e2e_terraform_plan_no_payload_allow
    kind="terraform.plan", payload={}
    → Pass=true

3.  e2e_destructive_empty_payload_deny
    kind="terraform.apply", payload={}
    → Pass=false, RuleIDs contains "ops.insufficient_context"

4.  e2e_kubectl_delete_no_namespace_deny
    kind="kubectl.delete", payload={resource:"pod"} (NO namespace)
    → Pass=false, RuleIDs contains "ops.insufficient_context"

5.  e2e_kubectl_delete_protected_ns_deny
    kind="kubectl.delete", payload={namespace:"kube-system", resource:"pod"}
    → Pass=false, RuleIDs contains "k8s.protected_namespace"

6.  e2e_kubectl_delete_safe_ns_allow
    kind="kubectl.delete", payload={namespace:"default", resource:"pod"}
    → Pass=true

7.  e2e_terraform_public_s3_deny
    kind="terraform.apply", payload with s3_public_access_block missing required settings
    → Pass=false, RuleIDs contains "terraform.s3_public_access"

8.  e2e_terraform_iam_wildcard_deny
    kind="terraform.apply", payload with iam_policy_statements Action:* Resource:*
    → Pass=false, RuleIDs contains "terraform.iam_wildcard_policy"

9.  e2e_privileged_container_deny
    kind="kubectl.apply", payload={namespace:"default", resource:"deployment",
      containers:[{image:"nginx", security_context:{privileged:true}}]}
    → Pass=false, RuleIDs contains "k8s.privileged_container"

10a. e2e_workload_apply_no_containers_deny
     kind="kubectl.apply", payload={namespace:"default", resource:"deployment"}
     → Pass=false, RuleIDs contains "ops.insufficient_context"

10b. e2e_nonworkload_apply_no_containers_allow
     kind="kubectl.apply", payload={namespace:"default", resource:"configmap"}
     → Pass=true

11. e2e_unknown_tool_destructive_deny
    kind="pulumi.up", payload={}
    → Pass=false, RuleIDs contains "ops.unknown_destructive"

12. e2e_valid_terraform_apply_allow
    kind="terraform.apply", payload={destroy_count:0, total_changes:1,
      resource_types:["aws_instance"]}
    → Pass=true

13. e2e_terraform_counts_only_deny
    kind="terraform.apply", payload={destroy_count:0}
    → Pass=false, RuleIDs contains "ops.insufficient_context"

14. e2e_unknown_tool_safe_suffix_deny
    kind="pulumi.plan", payload={}
    → Pass=false, RuleIDs contains "ops.unknown_destructive"

15. e2e_unknown_tool_breakglass_allow
    kind="crossplane.apply", payload={}, risk_tags=["breakglass"]
    → Pass=true (breakglass overrides unknown_destructive)
    Note: verify result includes breakglass warning

16. e2e_workload_fake_empty_containers_deny
    kind="kubectl.apply", payload={namespace:"default", resource:"deployment",
      containers:[{}]}
    → Pass=false, RuleIDs contains "ops.insufficient_context"

17. e2e_terraform_detail_nonsense_deny
    kind="terraform.apply", payload={destroy_count:0, total_changes:1,
      security_group_rules:[{foo:"bar"}]}
    → Pass=false, RuleIDs contains "ops.insufficient_context"

18. e2e_terraform_s3_block_empty_object_deny
    kind="terraform.apply", payload={destroy_count:0, total_changes:1,
      s3_public_access_block:{}}
    → Pass=false, RuleIDs contains "ops.insufficient_context"

19. e2e_added_tool_no_clause_deny
    kind="crossplane.apply" (temporarily add to destructive_operations
    in test — or accept that without clause ops.insufficient_context fires)
    payload={anything:true}
    → Pass=false, RuleIDs contains "ops.insufficient_context"
    Note: crossplane.apply is NOT in default destructive_operations,
    so this test needs a custom params override OR tests unknown_destructive.
    Simplest: test that unknown tool + non-empty payload → deny (unknown_destructive)
    because crossplane is not in destructive_operations.

20. e2e_added_tool_read_op_auto_allow
    kind="kubectl.version", payload={}
    → Pass=true (kubectl is known prefix, "version" is safe suffix)
```

**Building test scenarios:**

Use this pattern for Action construction (match the existing OPA input shape):

```go
scenario.Action{
    Kind:    "kubectl.delete",
    Target:  map[string]interface{}{"namespace": "kube-system"},
    Payload: map[string]interface{}{
        "namespace":      "kube-system",
        "resource":       "pod",
        "resource_count": 1,
    },
    RiskTags: []string{},
}
```

Note: `defaults.action_namespace()` reads namespace from either
`action.payload.namespace` or `action.target.namespace`. For consistency,
put namespace in both Target and Payload.

For breakglass tests, set `RiskTags: []string{"breakglass"}`.

**Done when:** `go test ./pkg/validate/ -run TestKillswitch -v` — all 21 tests pass.

---

## Task 6 (optional): Create deny_truncated_context.rego

**File to create:**
`policy/bundles/ops-v0.1/evidra/policy/rules/deny_truncated_context.rego`

This is lower priority (adapter flow only). Content from plan v0.6 Step 5.

```rego
package evidra.policy

import data.evidra.policy.defaults as defaults

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

Note: `is_destructive` is defined in `deny_insufficient_context.rego`.
OPA evaluates all files in the same package together, so this works.
However, if you get a "conflicting rule" error, move `is_destructive`
to `defaults.rego` instead.

**Done when:** test with `terraform.apply` + `resource_changes_truncated: true`
→ deny with `ops.truncated_context`.

---

## Verification checklist

After all tasks:

```bash
# All existing tests still pass
go test ./...

# New kill-switch tests pass
go test ./pkg/validate/ -run TestKillswitch -v

# Build succeeds
go build ./...

# Quick manual check: MCP tool description updated
grep -A5 "REQUIRED before executing" pkg/mcpserver/server.go
```

Expected state:
- 3 new rego files in rules/
- 2 edited JSON files (params + hints)
- 1 edited Go file (server.go description)
- 1 new test file (killswitch_test.go)
- 21 new tests, all green
- Zero changes to existing rules, helpers, or decision aggregator
