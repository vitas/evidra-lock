# Evidra — Validation Engine Architecture

Complete description of the validation engine. Covers current state (v0.1)
and planned v0.2.0 changes.

---

## 1. Architecture overview

```
                   ┌──────────────┐
                   │  Entry Point │
                   └──────┬───────┘
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
         MCP Server   CLI (file)   CLI (invocation)
              │           │           │
              ▼           ▼           ▼
      ToolInvocation   Scenario    ToolInvocation
              │           │           │
              ▼           │           ▼
   invocationToScenario   │   invocationToScenario
              │           │           │
              └───────────┼───────────┘
                          ▼
               EvaluateScenario()
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
         Load Policy   Per-action   Write Evidence
         (bundle)      OPA eval     (hash chain)
                          │
                          ▼
                    Aggregate results
                          │
                          ▼
                   Result + ActionResults[]
```

**Key files:**

| Component | File |
|---|---|
| MCP entry | `pkg/mcpserver/server.go` |
| Validation pipeline | `pkg/validate/validate.go` |
| OPA runtime | `pkg/runtime/runtime.go` |
| OPA engine | `pkg/policy/policy.go` |
| Policy bundle | `policy/bundles/ops-v0.1/` |
| Evidence store | `pkg/evidence/` |
| Bundle embed | `bundleembed.go` |

---

## 2. Input formats

### 2a. Scenario (native, multi-action)

```go
type Scenario struct {
    ScenarioID string
    Actor      Actor     // {Type, ID, Origin}
    Source     string
    Timestamp  time.Time
    Actions    []Action  // one or more actions
}

type Action struct {
    Kind     string                 // "kubectl.apply", "terraform.destroy"
    Target   map[string]interface{} // {"namespace": "prod"}
    Intent   string                 // Log-only. Recorded in evidence. Not evaluated by policy.
    Payload  map[string]interface{} // validation data
    RiskTags []string               // ["breakglass", "change-approved"]
}
```

Scenario is a multi-action envelope. Each action is evaluated separately.
If any action is denied, the entire scenario is denied.

### 2b. ToolInvocation (MCP/API)

```go
type ToolInvocation struct {
    Actor       Actor                  // type, id, origin (all required)
    Tool        string                 // "kubectl" (required)
    Operation   string                 // "apply" (required)
    Params      map[string]interface{} // target, payload, risk_tags, scenario_id
    Context     map[string]interface{} // source, intent, scenario_id
    Environment string                 // "production", "staging"
}
```

**Conversion ToolInvocation → Scenario:**
- `Tool` + `Operation` → `Action.Kind` (`"kubectl.apply"`)
- `Params["target"]` → `Action.Target`
- `Params["payload"]` → `Action.Payload`
- `Params["risk_tags"]` → `Action.RiskTags`

**Strict validation (`ValidateStructure()`):**
- All actor fields required (type, id, origin)
- tool and operation required
- params required (not nil)
- Allowed params keys: `target`, `payload`, `risk_tags`, `scenario_id`
- Allowed context keys: `source`, `intent`, `scenario_id`
- target/payload must be `map[string]interface{}` if present
- risk_tags must be `[]string` if present
- Unknown keys → reject

**v0.2.0 BREAKING:** MCP accepts only `params.action` (single).
`params.actions[]` from MCP → `invalid_input` reject.
CLI/file scenarios still support multi-action.

---

## 3. Validation pipeline (6 steps)

### Step 1: Load policy bundle

```
resolveBundlePath():
    1. --bundle flag (explicit)
    2. EVIDRA_BUNDLE_PATH env var
    3. config.DefaultBundlePath (embedded)
    4. fallback: separate policy/data paths

BundleSource:
    LoadPolicy() → map[filename][]byte (all .rego files)
    LoadData()   → []byte (merged data.json)
    PolicyRef()  → content hash

runtime.NewEvaluator(src):
    policy.NewOPAEngine(modules, data)
        rego.PrepareForEval() ← OPA compiles all rego at once
```

**Bundle layout:**

```
policy/bundles/ops-v0.1/
├── .manifest                          # revision, roots, profile_name
├── evidra/
│   ├── data/
│   │   ├── params/data.json           # configurable parameters
│   │   └── rule_hints/data.json       # actionable hints per rule ID
│   └── policy/
│       ├── policy.rego                # entry: data.evidra.policy.decision
│       ├── decision.rego              # aggregator: deny[] + warn[] → decision
│       ├── defaults.rego              # helpers
│       └── rules/                     # 26 rule files (deny_* + warn_*)
└── tests/                             # OPA unit tests (excluded at load)
```

**Embedded bundle:** `//go:embed all:policy/bundles/ops-v0.1` compiles
the bundle into the binary. Zero configuration at startup.

### Step 2: Per-action evaluation loop

```go
for i, action := range sc.Actions {
    tool, operation := splitKind(action.Kind)

    inv := ToolInvocation{
        Params: {
            "action": {                          // ONE action
                "kind":      action.Kind,
                "target":    action.Target,
                "payload":   action.Payload,
                "risk_tags": action.RiskTags,
            },
        },
    }

    decision := runtimeEval.EvaluateInvocation(inv)

    actionResults[i] = ActionResult{...}  // v0.2.0
}
```

**Critical:** each iteration creates a separate ToolInvocation with ONE
action in `params.action`. `buildActionList()` extracts it into
`input.actions[0]`. Each OPA eval sees exactly one action.
Per-action breakdown is exact.

### Step 3: OPA input construction

```json
{
    "actor":       {"type": "agent", "id": "claude-code", "origin": "mcp"},
    "tool":        "kubectl",
    "operation":   "apply",
    "environment": "production",
    "actions": [
        {
            "kind":      "kubectl.apply",
            "target":    {"namespace": "default"},
            "payload":   {"containers": [{"image": "nginx:1.25"}]},
            "risk_tags": []
        }
    ]
}
```

**`input.actions[]`** is the source of truth. All OPA rules read
`input.actions[_]`, never `input.params`.

### Step 4: OPA evaluation

```
data.evidra.policy.decision
    │
    └─ decision_impl.decision
        │
        ├─ denies := [{label, msg} | deny[label] = msg]
        │   ├─ deny_insufficient_context    → ops.insufficient_context
        │   ├─ deny_unknown_destructive     → ops.unknown_destructive
        │   ├─ deny_truncated_context       → ops.truncated_context
        │   ├─ deny_kube_system             → k8s.protected_namespace
        │   ├─ deny_mass_delete             → ops.mass_delete
        │   ├─ deny_privileged_container    → k8s.privileged_container
        │   ├─ deny_terraform_metadata_only → ops.terraform_metadata_only
        │   └─ ... (all deny_*.rego)
        │
        ├─ warnings := [{label, msg} | warn[label] = msg]
        │
        ├─ allow := (count(denies) == 0)
        │
        ├─ risk_level := high|medium|low
        │
        ├─ hits := dedupe(deny_labels + warn_labels)
        │
        ├─ hints := [hint | label in hits, hint in rule_hints[label]]
        │
        └─ decision := {allow, risk_level, reason, reasons, hits, hints}
```

**Aggregation:** if any deny fires → `allow = false`.
Warnings do NOT block — they add labels to `hits`.
Hints loaded from `rule_hints/data.json` by label.

### Step 5: Result aggregation

**v0.2.0 BREAKING:**

```go
result := Result{
    Pass:          allActionsPass,
    RiskLevel:     worstRiskLevel,
    RuleIDs:       union(actionResults[*].RuleIDs),   // summary
    Reasons:       union(actionResults[*].Reasons),   // summary
    Hints:         union(actionResults[*].Hints),     // summary
    ActionResults: actionResults,                      // always present, source of truth
}
```

`ActionResults[]` is always present. Flat fields are summary only
(deduped union of all action results).

### Step 6: Evidence recording

```go
rec := EvidenceRecord{
    EventID:        "evt-<unix_nano>",
    Timestamp:      time.Now().UTC(),
    PolicyRef:      contentHash,
    BundleRevision: "ops-v0.1.0-dev",
    ProfileName:    "ops-v0.1",
    InputHash:      sha256(scenario),    // v0.3.0: canonical hash
    PreviousHash:   lastRecordHash,      // hash chain link
    Hash:           thisRecordHash,
    PolicyDecision: {Allow, RiskLevel, Reason, Reasons, Hints, RuleIDs},
}
```

**Hash chain:** each record contains `PreviousHash`. Chain integrity
verified via `evidra evidence verify`.

---

## 4. Rules

### 4a. Kill-switch (baseline, always active)

| Rule | ID | Checks |
|---|---|---|
| deny_unknown_destructive | `ops.unknown_destructive` | Unknown tool + not safe read |
| deny_insufficient_context | `ops.insufficient_context` | Destructive op missing required context |
| deny_truncated_context | `ops.truncated_context` | Payload has truncation flags |
| deny_kube_system | `k8s.protected_namespace` | Namespace in restricted list |
| deny_mass_delete | `ops.mass_delete` | kubectl.delete above threshold |
| deny_prod_without_approval | `ops.unapproved_change` | Protected namespace, no approval tag |
| deny_terraform_metadata_only | `ops.terraform_metadata_only` | Ops profile, no deep fields |
| warn_breakglass | `ops.breakglass_used` | Breakglass tag (allow + logged) |
| warn_autonomous_execution | `ops.autonomous_execution` | Agent + destructive + no approval |

### 4b. Ops-layer (domain rules, profile=ops)

**Kubernetes:** privileged container, host namespace, run as root,
hostpath mount, dangerous capabilities, public exposure, mutable
image tag, no resource limits.

**Terraform:** SG open world, IAM wildcard, S3 public access,
S3 no encryption, S3 no versioning, IAM wildcard policy/principal.

**ArgoCD:** autosync, dangerous sync, wildcard destination.

### 4c. Read-only bypass

Safe suffixes: `get`, `list`, `describe`, `show`, `diff`, `plan`, `status`, `version`.

Known prefixes derive from `ops.destructive_operations`. Adding
`crossplane.apply` to the list automatically allows `crossplane.get`.

### 4d. Parameter system

All parameters are per-environment via `resolve_param(key)`:
`by_env[input.environment]` → `by_env["default"]` fallback.

---

## 5. Output format

### v0.2.0 output (BREAKING)

```json
{
    "pass": false,
    "risk_level": "high",
    "evidence_id": "evt-1234567890",
    "rule_ids": ["ops.insufficient_context"],
    "reasons": ["terraform.apply lacks required context..."],
    "hints": ["Provide: resource_types..."],
    "action_results": [
        {
            "index": 0,
            "kind": "terraform.apply",
            "pass": false,
            "risk_level": "high",
            "rule_ids": ["ops.insufficient_context"],
            "reasons": ["terraform.apply lacks required context. Provide: resource_types..."],
            "hints": ["{\"payload\":{\"resource_types\":[\"...\"]}}"]
        }
    ],
    "source": "local",
    "policy_ref": "sha256:abc..."
}
```

**`action_results[]`** — always present, source of truth.
**Flat fields** — summary (deduped union).

---

## 6. Error handling

All errors in evaluation result in deny. Evidra never returns
`allow: true` on evaluation failure.

| Error | Sentinel | Result |
|---|---|---|
| Invalid input | `ErrInvalidInput` | reject |
| Bundle/OPA failure | `ErrPolicyFailure` | deny |
| Evidence write failure | `ErrEvidenceWrite` | error (not silent) |
| OPA eval error | — | `{Allow: false, RiskLevel: "high"}` |

---

## 7. Design decisions

| Decision | Rationale |
|---|---|
| **Fail-closed** | Default deny. Allow requires passing ALL deny rules |
| **Deterministic** | No runtime API calls, network, or LLM in eval loop |
| **Per-action eval** | Each action evaluated separately. Breakdown exact |
| **Multi-action = any deny** | One deny → entire scenario denied |
| **No execution** | Evidra does not execute commands. Called BEFORE execution |
| **Evidence = side-effect** | Written AFTER evaluation. Write failure is not silent |
| **Engine = pipeline** | All business logic in policy/data. Engine is minimal |
| **actions[] = source of truth** | Rules read input.actions, not input.params |
