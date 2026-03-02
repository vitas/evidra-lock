# Evidra Engine v2 — Implementation Plan (Canonicalizer + Golden Policies + Rule Overrides)

## Status

Proposed (clean cut allowed: no public release yet)

---

## 1. Goals (v2)

### P0 (must ship)
1) **Fix the original logic gap**: native K8s manifests from agents must evaluate correctly (no false `ops.insufficient_context` due to shape/casing).
2) Introduce **Golden Policies** as the curated destructive-deny set backing the **Agent Kill Switch** feature.
3) Add **Rule Disable Overrides** (override-only) for operational control without touching rule logic.
4) Prevent future drift with CI invariants.

### P1 (nice to have in same PR if cheap)
- Improve MCP `validate` schema/description to truthfully reflect “native or flat accepted”.

---

## 2. Architecture Invariants (enforce in CI)

**Invariant A — Canonicalization boundary**
- No rule may reference `input.actions` directly.
- Rules must only read `defaults.actions`.

**Invariant B — Format knowledge isolation**
- Kubernetes shape paths (`spec.template.spec`, camelCase, etc.) exist only in `canonicalize.rego`.

**Invariant C — Central rule disable**
- Every rule must call `defaults.rule_enabled(rule_id)`.

---

## 3. Concrete Repo Changes (policy bundle)

Bundle root:
- `policy/bundles/ops-v0.1/`

### 3.1 Add canonicalizer file (NEW)

Create:
- `policy/bundles/ops-v0.1/evidra/policy/canonicalize.rego`

Package:
- `package evidra.policy.defaults`

Exports:
- `actions := [normalize_action(a) | a := input.actions[_]]`

Minimum supported K8s shapes:
- Flat: `payload.containers[]` already present
- Deployment-like: `payload.spec.template.spec`
- Pod: `payload.spec`
- CronJob: `payload.spec.jobTemplate.spec.template.spec`

Minimum normalized fields (projection only):
- `namespace` (from `metadata.namespace` or existing `namespace`)
- `resource` (lower(kind))
- `containers`, `init_containers`
- `volumes`
- `host_pid`, `host_ipc`, `host_network`
- container `securityContext` → `security_context` with key renames for:
  - privileged
  - runAsUser/runAsNonRoot/runAsGroup
  - allowPrivilegeEscalation
  - readOnlyRootFilesystem
  - capabilities (pass through object as-is; only key rename at container level)

Non-K8s tools:
- pass-through payload (Terraform/ArgoCD/Helm remain “flat enough” for v2)

**Important:** canonicalizer is a projection layer, not a lossless manifest transformer.

---

### 3.2 Switch all rules to `defaults.actions` (bulk edit)

Current state:
- 26/27 rule files reference `input.actions` directly.

Files (current) referencing `input.actions`:
- `policy/bundles/ops-v0.1/evidra/policy/decision.rego`
- `policy/bundles/ops-v0.1/evidra/policy/rules/*` (most)

Change pattern in every rule:
```rego
# before
action := input.actions[_]

# after
action := defaults.actions[_]
```

Also update `decision.rego` loops that iterate `input.actions[i]` to use `defaults.actions[i]`.

Note:
- `warn_autonomous_execution.rego` already does not use `input.actions` (keep as-is, but ensure it reads `defaults.actions` if it reads actions at all).

---

### 3.3 Simplify defaults helpers (remove multi-format walking)

After canonicalization, remove the B-style “format-agnostic” helpers from:
- `policy/bundles/ops-v0.1/evidra/policy/defaults.rego`

Examples:
- `all_containers(payload)` should be simple concat of:
  - `payload.containers`
  - `payload.init_containers`
- `action_namespace(a)` should read:
  - `a.payload.namespace` (and optionally `a.target.namespace` if you want to keep that secondary channel)

No references to:
- `spec.template.spec.*`
- `metadata.namespace`
- `securityContext` (camelCase)
in defaults after v2.

---

## 4. Rule Disable Overrides (rename; override-only)

### 4.1 Add policy data file (NEW)

Create a new data file (keep existing params untouched):
- `policy/bundles/ops-v0.1/evidra/data/policy/data.json`

```json
{
  "evidra": {
    "policy": {
      "rule_overrides": {
        "disabled_rules": []
      }
    }
  }
}
```

### 4.2 Add helper in defaults.rego

In `policy/bundles/ops-v0.1/evidra/policy/defaults.rego`:

```rego
rule_enabled(rule_id) := enabled {
  ov := object.get(data.evidra.policy, "rule_overrides", {})
  disabled := object.get(ov, "disabled_rules", [])
  enabled := not (rule_id in disabled)
}
```

### 4.3 Guard all rules (bulk edit)

Every deny/warn rule must have:

```rego
rule_id := "<stable.id>"
defaults.rule_enabled(rule_id)
```

Best practice:
- `hit.id == rule_id` always

---

## 5. Golden Policies Registry (Agent Kill Switch contract)

### 5.1 Add registry to policy data (same NEW file)

Extend `policy/bundles/ops-v0.1/evidra/data/policy/data.json`:

```json
{
  "evidra": {
    "policy": {
      "golden": {
        "rule_ids": [
          "k8s.privileged_container",
          "k8s.host_pid",
          "k8s.host_network",
          "ops.mass_delete",
          "tf.destroy_many"
        ]
      },
      "rule_overrides": {
        "disabled_rules": []
      }
    }
  }
}
```

This list is the explicit contract for the Agent Kill Switch feature.
Rules themselves do not encode “golden-ness”.

Optional helper:
```rego
is_golden(rule_id) {
  ids := object.get(object.get(data.evidra.policy, "golden", {}), "rule_ids", [])
  rule_id in ids
}
```

(Useful later for reporting/UI; not required for enforcement.)

---

## 6. Tests (OPA + Go + E2E)

### 6.1 OPA unit tests: canonicalizer (NEW)

Add:
- `policy/bundles/ops-v0.1/tests/canonicalize_test.rego`

Must include:
1) Flat passthrough (semantics)
2) Deployment-like manifest extraction
3) Pod manifest extraction
4) CronJob extraction
5) securityContext camelCase → snake_case output keys
6) End-to-end: native privileged container denies with `k8s.privileged_container` (NOT `ops.insufficient_context`)

### 6.2 Update existing rule tests (minimal changes)

Existing tests under:
- `policy/bundles/ops-v0.1/tests/*_test.rego`

Because rules now use `defaults.actions`, any tests that build `input.actions` still work.
But if any test asserts internal structure, update accordingly.

### 6.3 Add rule override test (NEW)

In a new test file (or extend contract test):
- verify that when `data.evidra.policy.rule_overrides.disabled_rules` contains a rule id, the hit is absent.

### 6.4 Go tests: embedded bundle cache invalidation (P0 recommended)

Current bug class: embedded bundle extraction is cached forever if `.manifest` exists.
File:
- `cmd/evidra-mcp/main.go` (`extractEmbeddedBundleCached`)

**Fix in v2:**
- include a version marker in cache, e.g. write/read:
  - `.embedded_commit` = `version.Commit` OR
  - `.embedded_sha256` computed from embedded bundle files
- re-extract if marker differs

Update tests:
- `cmd/evidra-mcp/embedded_bundle_mcp_test.go` to assert cache invalidates across version marker changes.

This prevents “policy changes not taking effect” in offline/fallback mode.

### 6.5 E2E: MCP stdio integration (extend)

Tests under:
- `cmd/evidra-mcp/test/stdio_integration_test.go`

Add a case:
- send **native K8s Deployment manifest** with privileged container
- assert deny reason is the privileged rule id (not insufficient_context)

---

## 7. MCP Protocol: what to leverage in v2

### 7.1 What MCP gives us (use it as P1 optimization)
- Tool discovery and `InputSchema` strongly influence agent output quality.
- Tool description + server instructions reduce retries and unsafe loops.

### 7.2 What MCP does NOT guarantee (do NOT rely on)
- Schema adherence
- snake_case / flattening
- reading instructions
- not sending raw manifests

Therefore:
- Canonicalizer is P0 enforcement boundary.

### 7.3 Concrete MCP updates (P1)

File:
- `pkg/mcpserver/server.go` (validate tool definition)

Update `params` description and/or add structure for `params.payload`:

- Explicitly state:
  - “payload may be a native K8s manifest or flat fields; evidra normalizes internally”
- Keep it short; do not over-specify every K8s field path (canonicalizer handles it).

---

## 8. Rollout / Sequencing (single clean PR is fine)

Recommended order (fastest path):
1) Add `canonicalize.rego` exporting `defaults.actions`
2) Bulk switch rules + decision to `defaults.actions`
3) Simplify defaults helpers
4) Add `data/policy/data.json` with `golden` + `rule_overrides`
5) Add `rule_enabled` guard to all rules
6) Add OPA tests (canonicalizer + override)
7) Fix embedded bundle cache invalidation + tests
8) Update MCP schema/description

---

## 9. Definition of Done

- `opa test policy/bundles/ops-v0.1/ -v` passes
- Native Deployment privileged manifest:
  - denies with `k8s.privileged_container`
  - does NOT deny with `ops.insufficient_context`
- No policy file contains `input.actions` (grep-enforced)
- All rules have `defaults.rule_enabled(rule_id)`
- Offline/fallback mode loads the updated embedded bundle after rebuild (cache invalidation verified)
- MCP validate tool description truthfully states native/flat accepted

---

## 10. Notes on Scope Control

To keep v2 maintainable:
- Canonicalizer supports only:
  - Pod
  - Deployment-like workloads
  - CronJob
- CRDs and “embedded podSpec in arbitrary paths” require an explicit follow-up design (v3).

---
