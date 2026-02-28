# Engine v0.2.0 — Implementation Instructions

Breaking release. 4 changes. ~95 lines total.

---

## Change 1: MCP single-action only — reject params.actions

**File:** `pkg/policy/policy.go`

Replace `buildActionList()` to reject `params.actions` from MCP:

```go
func buildActionList(params map[string]interface{}, origin string) ([]map[string]interface{}, error) {
    // Primary: single action (MCP and CLI)
    if raw, ok := params["action"]; ok {
        if action, ok2 := normalizeAction(raw); ok2 {
            return []map[string]interface{}{action}, nil
        }
    }

    // params.actions from MCP → reject
    if _, hasActions := params["actions"]; hasActions {
        if isMCPOrigin(origin) {
            return nil, fmt.Errorf("MCP must use params.action (single action); " +
                "for multiple operations call validate once per action")
        }
    }

    // params.actions from CLI/file → still supported
    if raw, ok := params["actions"]; ok {
        if arr, ok2 := raw.([]interface{}); ok2 {
            var actions []map[string]interface{}
            for _, item := range arr {
                if action, ok3 := normalizeAction(item); ok3 {
                    actions = append(actions, action)
                }
            }
            return actions, nil
        }
    }

    return nil, nil
}

func isMCPOrigin(origin string) bool {
    return origin == "mcp" || origin == "mcp-server"
}
```

**Update callers:** `buildActionList` is called in `Engine.Evaluate()`.
Pass `inv.Actor.Origin` as the origin parameter.

**Test:**

```go
func TestBuildActionList_RejectActionsFromMCP(t *testing.T) {
    params := map[string]interface{}{
        "actions": []interface{}{map[string]interface{}{"kind": "kubectl.get"}},
    }
    _, err := buildActionList(params, "mcp")
    require.Error(t, err)
    require.Contains(t, err.Error(), "single action")
}

func TestBuildActionList_AllowActionsFromCLI(t *testing.T) {
    params := map[string]interface{}{
        "actions": []interface{}{map[string]interface{}{"kind": "kubectl.get"}},
    }
    actions, err := buildActionList(params, "cli")
    require.NoError(t, err)
    require.Len(t, actions, 1)
}

func TestBuildActionList_SingleActionFromMCP(t *testing.T) {
    params := map[string]interface{}{
        "action": map[string]interface{}{"kind": "kubectl.apply"},
    }
    actions, err := buildActionList(params, "mcp")
    require.NoError(t, err)
    require.Len(t, actions, 1)
}
```

---

## Change 2: ActionResults always present

**File:** `pkg/validate/validate.go`

### Add types:

```go
type ActionResult struct {
    Index     int      `json:"index"`
    Kind      string   `json:"kind"`
    Pass      bool     `json:"pass"`
    RiskLevel string   `json:"risk_level"`
    RuleIDs   []string `json:"rule_ids"`
    Reasons   []string `json:"reasons"`
    Hints     []string `json:"hints"`
}
```

### Update Result struct:

```go
type Result struct {
    Pass        bool     `json:"pass"`
    RiskLevel   string   `json:"risk_level"`
    EvidenceID  string   `json:"evidence_id,omitempty"`
    EvidenceIDs []string `json:"evidence_ids,omitempty"`
    RequestIDs  []string `json:"request_ids,omitempty"`

    // Summary fields (deduped union of all ActionResults)
    Reasons     []string `json:"reasons"`
    RuleIDs     []string `json:"rule_ids"`
    Hints       []string `json:"hints"`

    // Per-action detail — always present, source of truth
    ActionResults []ActionResult `json:"action_results"`

    Source      string `json:"source"`
    PolicyRef   string `json:"policy_ref,omitempty"`
}
```

**Note:** `ActionResults` has NO `omitempty`. Always present.

### Update evaluateScenarioWithRuntime():

Inside the existing `for i, action := range sc.Actions` loop,
after `decision, err := runtimeEval.EvaluateInvocation(inv)`:

```go
    // Capture per-action result
    arRiskLevel := "low"
    if !decision.Allow {
        arRiskLevel = "high"
    } else if hasBreakglassTag(action) {
        arRiskLevel = "medium"
    }

    ar := ActionResult{
        Index:     i,
        Kind:      action.Kind,
        Pass:      decision.Allow,
        RiskLevel: arRiskLevel,
        RuleIDs:   sortedDedupeStrings(decision.Hits),
        Reasons:   sortedDedupeStrings(decision.Reasons),
        Hints:     sortedDedupeStrings(decision.Hints),
    }
    res.ActionResults = append(res.ActionResults, ar)
```

### Update scenarioEvaluation struct:

```go
type scenarioEvaluation struct {
    Pass          bool
    RiskLevel     string
    Reasons       []string
    RuleIDs       []string
    Hints         []string
    PolicyRef     string
    ActionResults []ActionResult  // add this
}
```

### Tests:

```go
func TestActionResults_AlwaysPresent(t *testing.T) {
    // Single action scenario
    sc := singleActionScenario("kubectl.get")
    result, err := validate.EvaluateScenario(ctx, sc, opts)
    require.NoError(t, err)
    require.NotNil(t, result.ActionResults)
    require.Len(t, result.ActionResults, 1)
    require.True(t, result.ActionResults[0].Pass)
}

func TestActionResults_MultiAction_Breakdown(t *testing.T) {
    sc := scenario.Scenario{
        ScenarioID: "multi-test",
        Actor: scenario.Actor{Type: "agent", ID: "test", Origin: "test"},
        Actions: []scenario.Action{
            {Kind: "kubectl.get"},
            {Kind: "terraform.apply", Payload: map[string]interface{}{}},
        },
    }
    result, err := validate.EvaluateScenario(ctx, sc, opts)
    require.NoError(t, err)

    require.False(t, result.Pass)
    require.Len(t, result.ActionResults, 2)

    // action 0: kubectl.get → pass
    require.True(t, result.ActionResults[0].Pass)
    require.Equal(t, "kubectl.get", result.ActionResults[0].Kind)
    require.Empty(t, result.ActionResults[0].RuleIDs)

    // action 1: terraform.apply empty → deny
    require.False(t, result.ActionResults[1].Pass)
    require.Equal(t, "terraform.apply", result.ActionResults[1].Kind)
    require.Contains(t, result.ActionResults[1].RuleIDs, "ops.insufficient_context")

    // Summary = union of all actions
    require.Contains(t, result.RuleIDs, "ops.insufficient_context")
}

func TestActionResults_SummaryIsUnion(t *testing.T) {
    // Verify flat fields are deduped union
    result := evaluateMultiDenyScenario()
    allRuleIDs := map[string]bool{}
    for _, ar := range result.ActionResults {
        for _, id := range ar.RuleIDs {
            allRuleIDs[id] = true
        }
    }
    for _, id := range result.RuleIDs {
        require.True(t, allRuleIDs[id], "summary rule_id %s not in action_results", id)
    }
}
```

---

## Change 3: Dynamic hints from data.json

**File:** `evidra/data/params/data.json`

Add to existing params:

```json
"ops.context_requirements": {
    "by_env": {
        "default": {
            "kubectl.delete": {
                "hint": "Provide: namespace",
                "skeleton": {"target": {"namespace": "..."}}
            },
            "kubectl.apply": {
                "hint": "Provide: namespace + containers[].image (for workloads)",
                "skeleton": {"target": {"namespace": "..."}, "payload": {"containers": [{"image": "..."}]}}
            },
            "terraform.apply": {
                "hint": "Provide: resource_types, or security_group_rules, or iam_policy_statements",
                "skeleton": {"payload": {"resource_types": ["..."]}}
            },
            "terraform.destroy": {
                "hint": "Provide: destroy_count (number)",
                "skeleton": {"payload": {"destroy_count": 0}}
            },
            "helm.*": {
                "hint": "Provide: namespace",
                "skeleton": {"target": {"namespace": "..."}}
            },
            "argocd.sync": {
                "hint": "Provide: app_name or sync_policy",
                "skeleton": {"payload": {"app_name": "..."}}
            }
        }
    }
}
```

**File:** `deny_insufficient_context.rego`

Update deny rule to use dynamic reasons. Keep deny as ID-only:

```rego
# deny is ID-only (stable for clients)
deny["ops.insufficient_context"] if {
    action := input.actions[_]
    is_destructive(action.kind)
    not has_sufficient_context(action)
}

# Dynamic reason with per-kind detail
insufficient_context_reason[msg] if {
    action := input.actions[_]
    is_destructive(action.kind)
    not has_sufficient_context(action)
    reqs := context_requirements(action.kind)
    msg := sprintf("%s lacks required context. %s", [action.kind, reqs.hint])
}

# Payload skeleton as hint
insufficient_context_hint[s] if {
    action := input.actions[_]
    is_destructive(action.kind)
    not has_sufficient_context(action)
    reqs := context_requirements(action.kind)
    s := json.marshal(reqs.skeleton)
}

context_requirements(kind) := req if {
    reqs := defaults.resolve_param("ops.context_requirements")
    req := reqs[kind]
}

context_requirements(kind) := req if {
    reqs := defaults.resolve_param("ops.context_requirements")
    not reqs[kind]
    prefix := sprintf("%s.*", [split(kind, ".")[0]])
    req := reqs[prefix]
}

context_requirements(kind) := {
    "hint": sprintf("No context requirements defined for %s", [kind]),
    "skeleton": {}
} if {
    reqs := defaults.resolve_param("ops.context_requirements")
    not reqs[kind]
    prefix := sprintf("%s.*", [split(kind, ".")[0]])
    not reqs[prefix]
}
```

**File:** `decision.rego`

Update to collect dynamic reasons and hints:

```rego
reasons := dedupe(array.concat(
    [entry.message | entry := denies[_]],
    [msg | msg := data.evidra.policy.insufficient_context_reason[msg]],
))

hints := dedupe(array.concat(
    [hint | label := hits[_]; hs := data.evidra.data.rule_hints[label]; hint := hs[_]],
    [h | h := data.evidra.policy.insufficient_context_hint[h]],
))
```

**IMPORTANT:** When deny changes from `deny["id"] = msg` to
`deny["id"]` (no message), update decision.rego denies collection:

For `ops.insufficient_context` specifically, the message now comes from
`insufficient_context_reason` instead of `deny[label] = message`.
Other deny rules keep `deny[label] = message` pattern unchanged.

Handle this by having the deny still produce a generic message:

```rego
deny["ops.insufficient_context"] = "Destructive operation lacks required context" if {
    ...
}
```

And the dynamic reason REPLACES it in the reasons array. Or simpler:
keep `deny[label] = msg` as-is, and just ADD the dynamic reasons to
the reasons array. The dynamic reason is more specific and will appear
alongside the generic one. Dedupe handles the rest.

**Test:**

```go
// In OPA test
test_insufficient_context_dynamic_hint if {
    d := data.evidra.policy.decision with input as {
        "actions": [{"kind": "terraform.apply", "payload": {}}],
    }
    d.allow == false
    some reason in d.reasons
    contains(reason, "Provide: resource_types")
}
```

---

## Change 4: Intent log-only comment

**File:** `pkg/scenario/schema.go`

```go
type Action struct {
    Kind     string                 `json:"kind"`
    Target   map[string]interface{} `json:"target"`
    Intent   string                 `json:"intent"` // Log-only. Recorded in evidence for audit. Not evaluated by policy.
    Payload  map[string]interface{} `json:"payload"`
    RiskTags []string               `json:"risk_tags"`
}
```

No functional change. Comment only.

---

## Verification

```bash
# All tests pass
go test ./...

# Specific new tests
go test ./pkg/validate/ -run TestActionResults -v
go test ./pkg/policy/ -run TestBuildActionList -v

# OPA tests
cd policy/bundles/ops-v0.1 && opa test . -v
```

## Done when

- [ ] `params.actions` from MCP origin → error
- [ ] `params.action` from MCP → works
- [ ] `result.ActionResults` always present (never nil, never omitempty)
- [ ] Flat `rule_ids/reasons/hints` = deduped union of action_results
- [ ] `ops.insufficient_context` deny → dynamic reason with kind + required fields
- [ ] `ops.insufficient_context` hints → payload skeleton JSON
- [ ] All existing tests pass
- [ ] New tests for: MCP reject, action breakdown, multi-action, dynamic hints
