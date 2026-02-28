# Evidra UI — Review & Implementation Instructions

Console, Dashboard, Docs. Landing page already fixed.

---

## Step 0: Decide canonical request format

**Finding:** Two paths exist with different formats:

| Path | Format | Where params.action is built |
|---|---|---|
| MCP server | `params.action = {kind, target, payload, risk_tags}` | `evaluateScenarioWithRuntime()` builds it |
| API server | Flat `params = {target, payload, risk_tags}` | `ValidateStructure()` rejects `params.action` |

**Problem:** API path sends flat params. `buildActionList(params)` looks
for `params.action` → not found → `input.actions` is nil in OPA → **all
rules that check `input.actions[_]` don't fire.** Most deny rules are
effectively dead for API requests.

This means the Dashboard "Try Validate" currently evaluates against OPA
with empty actions — most rules silently pass.

**Decision for v0.2.0:**

UI sends canonical format: `params.action`.

API backend must accept `params.action` — update `allowedParamKeys` in
`invocation.go` to include `"action"`.

```go
var allowedParamKeys = map[string]bool{
    KeyTarget: true, KeyPayload: true, KeyRiskTags: true,
    KeyScenarioID: true, "action": true,  // ← add
}
```

This is a backend change, not a UI change. But it's blocking for UI
correctness. Without it, UI sends `params.action` → backend rejects
with "unknown params key: action".

**Alternative (quicker):** keep flat params on API, but add server-side
conversion in `validate_handler.go`:

```go
// If flat params (no action key), build action from tool+operation+params
if _, hasAction := inv.Params["action"]; !hasAction {
    inv.Params["action"] = map[string]interface{}{
        "kind":    inv.Tool + "." + inv.Operation,
        "target":  inv.Params["target"],
        "payload": inv.Params["payload"],
        "risk_tags": inv.Params["risk_tags"],
    }
}
```

**Recommendation:** Do the server-side conversion (quick) so existing
curl users don't break, AND have UI send canonical format.

---

## Critical findings

### F1. API response has no wrapper (P0)

**UI expects:** `{ok, decision, evidence_record}`
**API returns:** `EvidenceRecord` directly (flat)

`result.evidence_record` doesn't exist. Works partially by accident
because `result.decision` is a field of EvidenceRecord.

### F2. OPA rules don't fire for API requests (P0)

Flat params → `buildActionList` returns nil → `input.actions` empty →
rules checking `input.actions[_]` silently pass. Dashboard "deny" test
cases may show "Allow" when they should deny.

### F3. Docs show fake response format (P0)

Examples show `{"ok": true, "decision": {...}, "evidence_record": {...}}`
which the API never returns.

### F4. No MCP setup anywhere (P0)

Primary use case (AI agent + MCP) has zero documentation in UI.

### F5. Dashboard result shows minimal info (P1)

Only: badge + single reason. Missing: all reasons[], hints[], rule_ids[].

### F6. No action_results support for v0.2.0 (P1)

v0.2.0 engine returns `action_results[]` in Result. UI should show
per-action breakdown with fallback to decision.* for older servers.

---

## Types (types/api.ts) — rewrite first

```typescript
// types/api.ts

export interface Actor {
    type: string;
    id: string;
    origin: string;
}

// Canonical request format (v0.2.0)
export interface ActionPayload {
    kind: string;                          // "kubectl.apply"
    target?: Record<string, unknown>;      // {namespace: "default"}
    payload?: Record<string, unknown>;     // {containers: [{image: "nginx"}]}
    risk_tags?: string[];
}

export interface ToolInvocation {
    actor: Actor;
    tool: string;
    operation: string;
    params: {
        action: ActionPayload;
    };
    environment?: string;
}

// Response: API returns EvidenceRecord directly (no wrapper)
export interface DecisionRecord {
    allow: boolean;
    risk_level: "low" | "medium" | "high";
    reason: string;
    reasons: string[];
    hints: string[];
    hits?: string[];
    rule_ids: string[];
}

export interface ActionResult {
    index: number;
    kind: string;
    pass: boolean;
    risk_level: string;
    rule_ids: string[];
    reasons: string[];
    hints: string[];
}

export interface ValidateResponse {
    // This IS the EvidenceRecord — no wrapper
    event_id: string;
    timestamp: string;
    server_id: string;
    tenant_id?: string;
    environment?: string;
    actor: Actor;
    tool: string;
    operation: string;
    input_hash: string;
    policy_ref: string;
    bundle_revision?: string;
    profile_name?: string;
    decision: DecisionRecord;
    // v0.2.0 — may or may not be present
    action_results?: ActionResult[];
    signing_payload: string;
    signature: string;
}

export interface KeyResponse {
    key: string;
    prefix: string;
    tenant_id: string;
}
```

**Key points:**
- `ValidateResponse` IS `EvidenceRecord` (no `ok`, no `evidence_record` wrapper)
- `ToolInvocation.params.action` is canonical (not flat target/payload)
- `action_results` optional (v0.2.0 engine only)
- `reasons`, `hints`, `rule_ids` default to `[]` not undefined

---

## Dashboard (Dashboard.tsx)

### D1. Fix buildInvocation — canonical format

```typescript
const buildInvocation = useCallback((): ToolInvocation | null => {
    if (mode === "advanced") {
        return jsonValid && jsonParsed
            ? (jsonParsed as unknown as ToolInvocation)
            : null;
    }

    const resolvedTool = tool === "custom" ? customTool : tool;
    const kind = `${resolvedTool}.${operation}`;

    // Build structured action
    const target: Record<string, unknown> = {};
    if (namespace.trim()) target.namespace = namespace.trim();

    const payload: Record<string, unknown> = {};
    for (const p of extraParams) {
        if (p.key.trim()) {
            payload[p.key.trim()] = tryParseJSON(p.value);
        }
    }

    return {
        actor: parseActor(actor),
        tool: resolvedTool,
        operation,
        params: {
            action: {
                kind,
                target: Object.keys(target).length > 0 ? target : undefined,
                payload: Object.keys(payload).length > 0 ? payload : undefined,
            },
        },
        environment: environment || undefined,
    };
}, [mode, tool, customTool, operation, namespace, actor, environment, extraParams, jsonParsed, jsonValid]);
```

### D2. Fix result handling — response IS evidence record

```typescript
// BEFORE (wrong):
result.decision.allow     // works by accident
result.evidence_record    // undefined!

// AFTER (correct):
result.decision.allow     // correct — decision is field of evidence record
result.decision.reasons   // all reasons
result.decision.hints     // all hints
result.decision.rule_ids  // rule IDs
// The whole `result` IS the evidence record
```

### D3. Result display with action_results support

```tsx
{result && (
    <div className="validate-result">
        {/* Summary */}
        <div className="result-summary">
            <Badge variant={result.decision.allow ? "allow" : "deny"}>
                {result.decision.allow ? "Allow" : "Deny"}
            </Badge>
            <Badge variant={result.decision.risk_level}>
                risk: {result.decision.risk_level}
            </Badge>
        </div>

        {/* Per-action breakdown (v0.2.0) or fallback */}
        {result.action_results && result.action_results.length > 0 ? (
            <div className="result-actions">
                <span className="result-label">
                    Actions ({result.action_results.length}):
                </span>
                {result.action_results.map((ar) => (
                    <div key={ar.index} className={`action-result ${ar.pass ? "" : "action-result--denied"}`}>
                        <div className="action-result-header">
                            <Badge variant={ar.pass ? "allow" : "deny"}>
                                {ar.pass ? "Allow" : "Deny"}
                            </Badge>
                            <code>{ar.kind}</code>
                        </div>
                        {ar.rule_ids.length > 0 && (
                            <div className="action-result-rules">
                                {ar.rule_ids.map((id) => (
                                    <code key={id} className="rule-id">{id}</code>
                                ))}
                            </div>
                        )}
                        {ar.reasons.length > 0 && (
                            <ul className="action-result-reasons">
                                {ar.reasons.map((r, i) => <li key={i}>{r}</li>)}
                            </ul>
                        )}
                        {ar.hints.length > 0 && (
                            <ul className="action-result-hints">
                                {ar.hints.map((h, i) => <li key={i}>{h}</li>)}
                            </ul>
                        )}
                    </div>
                ))}
            </div>
        ) : (
            <>
                {/* Fallback: flat decision fields (pre-v0.2.0 server) */}
                {result.decision.reason && result.decision.reason !== "ok" && (
                    <div className="result-reason">{result.decision.reason}</div>
                )}
                {(result.decision.rule_ids?.length ?? 0) > 0 && (
                    <div className="result-rules">
                        <span className="result-label">Rules:</span>
                        {result.decision.rule_ids.map((id) => (
                            <code key={id} className="rule-id">{id}</code>
                        ))}
                    </div>
                )}
                {(result.decision.reasons?.length ?? 0) > 0 && (
                    <div className="result-reasons">
                        <span className="result-label">Details:</span>
                        <ul>
                            {result.decision.reasons.map((r, i) => <li key={i}>{r}</li>)}
                        </ul>
                    </div>
                )}
                {(result.decision.hints?.length ?? 0) > 0 && (
                    <div className="result-hints">
                        <span className="result-label">Hints:</span>
                        <ul>
                            {result.decision.hints.map((h, i) => <li key={i}>{h}</li>)}
                        </ul>
                    </div>
                )}
            </>
        )}

        {/* Raw evidence record */}
        <button
            type="button"
            className="result-evidence-toggle"
            onClick={() => setShowEvidence(!showEvidence)}
        >
            {showEvidence ? "Hide" : "Show"} Raw Evidence
        </button>
        {showEvidence && (
            <CodeBlock code={JSON.stringify(result, null, 2)} />
        )}
    </div>
)}
```

### D4. Add terraform/argocd to tools

```typescript
const TOOL_OPTIONS = ["kubectl", "terraform", "helm", "argocd"];

const OP_OPTIONS: Record<string, string[]> = {
    kubectl: ["apply", "delete", "get", "patch", "rollout"],
    terraform: ["apply", "destroy", "plan"],
    helm: ["install", "upgrade", "uninstall", "rollback"],
    argocd: ["sync", "rollback", "terminate-op"],
};
```

### D5. Add resource kind + container image for kubectl.apply

Simple mode currently has namespace + arbitrary key-value params.
For kubectl.apply, kill-switch rules check `containers[].image` and
resource kind. Without these, `ops.insufficient_context` always fires.

Add conditional fields when tool=kubectl, operation=apply:

```tsx
{tool === "kubectl" && operation === "apply" && (
    <>
        <div className="form-field">
            <label htmlFor="dash-resource">Resource Kind</label>
            <select
                id="dash-resource"
                value={resourceKind}
                onChange={(e) => setResourceKind(e.target.value)}
            >
                <option value="">— none —</option>
                <option value="deployment">deployment</option>
                <option value="pod">pod</option>
                <option value="statefulset">statefulset</option>
                <option value="daemonset">daemonset</option>
                <option value="job">job</option>
                <option value="service">service</option>
                <option value="configmap">configmap</option>
            </select>
        </div>

        {isWorkloadKind(resourceKind) && (
            <div className="form-field">
                <label htmlFor="dash-image">Container Image</label>
                <input
                    id="dash-image"
                    type="text"
                    placeholder="e.g. nginx:1.25"
                    value={containerImage}
                    onChange={(e) => setContainerImage(e.target.value)}
                />
            </div>
        )}
    </>
)}
```

Update `buildInvocation()` to include these:

```typescript
const payload: Record<string, unknown> = {};
if (resourceKind) {
    payload.resource = resourceKind;
}
if (containerImage && isWorkloadKind(resourceKind)) {
    payload.containers = [{ image: containerImage }];
}
```

```typescript
const WORKLOAD_KINDS = new Set([
    "deployment", "pod", "statefulset", "daemonset", "replicaset", "job", "cronjob"
]);
function isWorkloadKind(kind: string): boolean {
    return WORKLOAD_KINDS.has(kind);
}
```

### D6. Fix advanced mode default JSON

```typescript
const defaultAdvanced = {
    actor: { type: "agent", id: "claude", origin: "claude-code" },
    tool: "kubectl",
    operation: "apply",
    params: {
        action: {
            kind: "kubectl.apply",
            target: { namespace: "default" },
            payload: { containers: [{ image: "nginx:1.25" }] },
        },
    },
    environment: "staging",
};
```

### D7. Better recent evaluations with event_id

```typescript
interface RecentEval {
    time: string;
    eventId: string;      // for "view evidence" link
    kind: string;         // "kubectl.apply"
    allow: boolean;
    riskLevel: string;
    ruleIds: string[];
    rawEvidence: object;  // full response for "View" click
}
```

In the table, add clickable event_id that expands raw JSON:

```tsx
<td>
    <button
        className="event-id-link"
        onClick={() => setExpandedEvent(
            expandedEvent === r.eventId ? null : r.eventId
        )}
    >
        {r.eventId.slice(0, 16)}...
    </button>
</td>
```

When expanded, show `<CodeBlock code={JSON.stringify(r.rawEvidence, null, 2)} />`.

### D8. API key UX

- Show key prefix in masked display: `ev1_abc1****`
- Add note: "Keys are per-tenant. Treat as secret. Rotate by creating new."
- Key shown once — only "regenerate" after.

---

## Console (Console.tsx)

### C1. Two-track onboarding: MCP (primary) + API

```
┌─────────────────────────────────────────┐
│           How will you use Evidra?       │
│                                         │
│  ┌──────────────┐  ┌──────────────┐    │
│  │  AI Agent     │  │  API / CI    │    │
│  │  (MCP)        │  │              │    │
│  └──────────────┘  └──────────────┘    │
└─────────────────────────────────────────┘
```

**Track A: MCP Agent (primary)**

```
Step 1: Install
  brew install evidra/tap/evidra-mcp
  # or
  go install samebits.com/evidra/cmd/evidra-mcp@latest

Step 2: Add to your editor

  Claude Desktop — add to claude_desktop_config.json:
  {
      "mcpServers": {
          "evidra": {
              "command": "evidra-mcp",
              "args": []
          }
      }
  }

  Claude Code:
  claude mcp add evidra evidra-mcp

  Cursor — add to .cursor/mcp.json:
  {
      "mcpServers": {
          "evidra": {
              "command": "evidra-mcp",
              "args": []
          }
      }
  }

Step 3: Verify
  Ask your agent: "What tools does evidra provide?"
  It should list the validate tool and evidence lookup tool.

  Then try: "Deploy nginx to default namespace"
  The agent will call evidra validate before kubectl apply.

Modes:
  ● Hosted (default): zero config, evidence signed by server
  ● Offline: evidra-mcp --offline --evidence-dir ~/.evidra
    All evaluation local, no network, evidence stored as hash-chain.
```

**Track B: API / CI**

Current Get Key flow (fixed curl example):

```bash
curl -X POST https://api.evidra.rest/v1/validate \
  -H "Authorization: Bearer ev1_YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "actor": {"type":"agent","id":"claude","origin":"curl"},
    "tool": "kubectl",
    "operation": "apply",
    "params": {
        "action": {
            "kind": "kubectl.apply",
            "target": {"namespace": "default"},
            "payload": {"containers": [{"image": "nginx:1.25"}]}
        }
    },
    "environment": "staging"
  }'
```

---

## Docs (Docs.tsx)

### K1. MCP Setup section (FIRST section)

Show Claude Desktop, Claude Code, Cursor configs. One example request.

Add deterministic verification test after "Verify":

```
Verify it works:
  1. "What tools does evidra provide?" → should list validate + get_event
  2. "Validate kubectl.delete in kube-system" → should DENY with rule k8s.protected_namespace

If step 2 shows Deny — everything is working correctly.
```

### K2. Fix response example — flat EvidenceRecord

```json
{
    "event_id": "evt_01JNPQ...",
    "timestamp": "2026-02-28T14:23:01Z",
    "server_id": "evidra-api-1",
    "actor": {"type": "agent", "id": "claude", "origin": "curl"},
    "tool": "kubectl",
    "operation": "apply",
    "input_hash": "sha256:...",
    "policy_ref": "bundle://ops-v0.1:sha256:...",
    "decision": {
        "allow": true,
        "risk_level": "low",
        "reason": "ok",
        "reasons": [],
        "hints": [],
        "rule_ids": []
    },
    "signing_payload": "evidra.v1\nevent_id=...",
    "signature": "base64..."
}
```

**Add prominent warning after response example:**

```
⚠️  HTTP 200 does not mean allow.
The API returns 200 for both allow and deny decisions.
Always check decision.allow:

  curl ... | jq '.decision.allow'

HTTP 4xx/5xx = request error (bad JSON, missing auth, server error).
HTTP 200 + decision.allow=false = policy denial (valid request, denied by policy).
```

### K3. Fix request example — canonical format

```json
{
    "actor": {"type": "agent", "id": "claude", "origin": "claude-code"},
    "tool": "kubectl",
    "operation": "apply",
    "params": {
        "action": {
            "kind": "kubectl.apply",
            "target": {"namespace": "default"},
            "payload": {"containers": [{"image": "nginx:1.25"}]}
        }
    },
    "environment": "staging"
}
```

### K4. Deny example with full details

```json
{
    "event_id": "evt_01JNPR...",
    "decision": {
        "allow": false,
        "risk_level": "high",
        "reason": "kube-system is a restricted namespace",
        "reasons": ["kube-system is a restricted namespace"],
        "hints": ["Do not deploy to kube-system unless breakglass."],
        "rule_ids": ["k8s.protected_namespace"]
    }
}
```

### K5. "Understanding Decisions" section

Table of common rule_ids, meaning, and fix.

### K6. CLI usage + evidence verification

```
evidra validate scenario.yaml
evidra-mcp
evidra-mcp --offline --evidence-dir ~/.evidra
```

**Evidence verification (one command):**

```bash
# Save the evidence record from response
curl ... > evidence.json

# Verify signature
evidra evidence verify --pubkey <(curl -s https://api.evidra.rest/v1/evidence/pubkey | jq -r .pem) evidence.json

# Or manually with OpenSSL:
jq -r .signing_payload evidence.json > payload.txt
jq -r .signature evidence.json | base64 -d > sig.bin
curl -s https://api.evidra.rest/v1/evidence/pubkey | jq -r .pem > pubkey.pem
openssl pkeyutl -verify -pubin -inkey pubkey.pem -rawin -in payload.txt -sigfile sig.bin
```

Highlight: "Evidence is Evidra's differentiator. Every decision is
cryptographically signed and independently verifiable."

### K7. MCP Troubleshooting section (NEW — important for support)

```
"Evidra tool not showing up"
  → Check config JSON path. Restart editor. Verify: evidra-mcp --version

"Permission denied" or "command not found"
  → Check PATH. brew: /opt/homebrew/bin. go install: ~/go/bin

"Agent doesn't call validate before kubectl"
  → Tool usage is not enforced by MCP. Include in agent instructions:
    "Always call evidra validate before any destructive operation."

"Deny: ops.insufficient_context"
  → Copy the skeleton from the hint. It shows exactly what fields to add.
    Example hint: {"payload":{"resource_types":["..."]}}

"Deny: ops.unknown_destructive"
  → Tool not in allowed list. Add it to ops.destructive_operations in data.json.

"All requests show Allow even when they should deny"
  → Verify server is using embedded bundle (not empty policy).
    Check: evidra-mcp --version should show bundle revision.
```

### K8. API Reference table update

```
POST /v1/validate     Bearer   Evaluate policy. Returns EvidenceRecord directly.
                                Deny = HTTP 200. Check decision.allow.
POST /v1/keys         none     Create API key. Returns {key, prefix, tenant_id}.
GET  /v1/evidence/pubkey  none  Ed25519 public key (PEM).
GET  /healthz         none     Always 200.
GET  /readyz          none     200 if DB up.
```

---

## Backend fix required (blocking for UI)

**File:** `internal/api/validate_handler.go`

Add server-side conversion so flat params still work AND params.action
works. Conversion runs BEFORE ValidateStructure().

```go
// After decoding inv, before ValidateStructure():
if _, hasAction := inv.Params["action"]; !hasAction && inv.Params != nil {
    old := inv.Params  // save before overwrite

    // Build canonical action from flat params
    action := map[string]interface{}{
        "kind": inv.Tool + "." + inv.Operation,
    }
    if t, ok := old["target"]; ok { action["target"] = t }
    if p, ok := old["payload"]; ok { action["payload"] = p }
    if r, ok := old["risk_tags"]; ok { action["risk_tags"] = r }

    // Rebuild params with action + preserved top-level keys
    inv.Params = map[string]interface{}{"action": action}

    // Preserve known non-action keys
    preserveKeys := []string{"scenario_id", "context", "evidence_level"}
    for _, key := range preserveKeys {
        if v, ok := old[key]; ok {
            inv.Params[key] = v
        }
    }
}
```

**Also update allowedParamKeys — strict canonical only:**

Since there are no external consumers yet, make ValidateStructure()
accept only the canonical format. Flat keys are handled by the
conversion above (before validation).

```go
// pkg/invocation/invocation.go
var allowedParamKeys = map[string]bool{
    "action":      true,
    KeyScenarioID: true,
    // flat keys (target, payload, risk_tags) accepted via handler conversion only
}
```

This means:
- UI/curl sends `params.action` → passes validation directly
- Old flat `params.target` → handler converts to `params.action` → passes
- Random unknown keys → rejected by ValidateStructure()

---

## Implementation order

1. **Backend:** Add server-side flat→action conversion in validate_handler.go
2. **Backend:** Add "action" to allowedParamKeys
3. **types/api.ts** — rewrite types
4. **Dashboard.tsx** — D1-D3 (canonical payload, result handling, action_results)
5. **Docs.tsx** — K1-K4 (MCP setup, fix examples)
6. **Console.tsx** — C1 (MCP track + API track)
7. **Polish:** D4-D7, K5-K8

## Done when

- [ ] Backend: flat params auto-converted to params.action (preserving scenario_id)
- [ ] Backend: allowedParamKeys = {action, scenario_id} only (flat via handler shim)
- [ ] Dashboard: sends canonical params.action format
- [ ] Dashboard: shows correct result (reasons, hints, rule_ids)
- [ ] Dashboard: action_results[] displayed when present (v0.2.0)
- [ ] Dashboard: fallback to decision.* when no action_results (pre-v0.2.0)
- [ ] Dashboard: kubectl.apply simple mode has resource kind + container image fields
- [ ] Dashboard: recent evals show event_id + clickable "View evidence"
- [ ] Docs: response example matches actual API (flat EvidenceRecord)
- [ ] Docs: request example uses canonical format
- [ ] Docs: MCP Setup with Claude Desktop/Code/Cursor + deterministic test
- [ ] Docs: MCP Troubleshooting section
- [ ] Docs: "HTTP 200 ≠ allow" warning + jq example
- [ ] Docs: evidence verify one-liner
- [ ] Console: MCP track as primary + offline mode noted
- [ ] Console: verify test = "delete in kube-system → DENY"
- [ ] Curl examples use canonical format with actor.origin
