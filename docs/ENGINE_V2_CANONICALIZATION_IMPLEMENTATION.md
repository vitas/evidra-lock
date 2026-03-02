# Evidra Engine v2 — Implementation Plan (Canonicalization Layers Only)

## Scope

This plan covers **only** the new engine logic for:

- canonicalization of agent-provided inputs (especially Kubernetes-native manifests)
- consistent policy evaluation on a stable internal schema

**Out of scope (do not touch):**
- API
- UI
- Evidence store
- Embedded bundle cache (already implemented)

---

## Background: Current Engine Behavior (what matters for v2)

- Go builds `input.actions` via `buildActionList(...)`.
- `payload` inside each action is currently passed **as-is** (agent format).
- Rules and `decision.rego` read `input.actions[_]` directly.
- Some format-agnostic helpers exist (Variant B style), e.g. walking K8s paths inside `defaults.rego`.

**Problem:** agents send domain-native K8s manifests (nested, camelCase), while rules expect a flat internal shape (top-level `containers`, `namespace`, snake_case). This causes false denies such as `ops.insufficient_context`.

---

## v2 Design: Canonicalization Boundary Inside OPA Bundle

### Key idea

Introduce a single canonicalization layer in Rego:

```
raw input.actions[_]
   ↓
canonicalize.rego (projection + casing normalization)
   ↓
defaults.actions[_]  (normalized, flat-only)
   ↓
ALL rules + decision.rego (flat-only)
```

### Invariants (must be enforced)

1) **No rule/decision may reference `input.actions` directly.**  
   All policy logic reads `defaults.actions`.

2) **All K8s shape/casing knowledge lives only in canonicalize.rego.**  
   No `spec.template...` walking in rules or defaults helpers.

3) Canonicalizer is a **projection layer**:
   - extract only security-relevant fields
   - do not validate Kubernetes schemas
   - do not attempt to preserve full manifests

---

## Canonicalizer Requirements

### Supported K8s payload shapes (minimum)

Canonicalizer must accept and normalize:

1) **Flat** (already internal shape)
   - `payload.containers[]` exists

2) **Workload template** (Deployment/StatefulSet/DaemonSet/etc.)
   - `payload.spec.template.spec`

3) **Bare Pod**
   - `payload.spec`

4) **CronJob**
   - `payload.spec.jobTemplate.spec.template.spec`

### Output: normalized flat schema (minimum fields)

Canonicalizer should produce a flat payload with:

- `namespace` (from `metadata.namespace` or existing `namespace`)
- `resource` (lowercased kind, e.g. `deployment`, `pod`, `cronjob`)
- `containers[]`
- `init_containers[]`
- `volumes[]`
- `host_pid`, `host_ipc`, `host_network`
- Container-level casing fix:
  - `securityContext` → `security_context`
  - selected keys:
    - `runAsUser` → `run_as_user`
    - `runAsNonRoot` → `run_as_non_root`
    - `runAsGroup` → `run_as_group`
    - `allowPrivilegeEscalation` → `allow_privilege_escalation`
    - `readOnlyRootFilesystem` → `read_only_root_filesystem`
  - keep other container keys as-is (name, image, etc.)

### Non-K8s tools

For tools other than `kubectl`/`oc`:
- pass-through payload unchanged (v2 scope is K8s canonicalization layer)

---

## Bundle Changes (Concrete)

### 1) Add canonicalizer module (NEW)

Create:

- `policy/bundles/ops-v0.1/evidra/policy/canonicalize.rego`

Package:

- `package evidra.policy.defaults`

Export:

- `actions := [canonicalize_action(a) | a := input.actions[_]]`

### 2) Switch decision + all rules to `defaults.actions`

Update:

- `policy/bundles/ops-v0.1/evidra/policy/decision.rego`
- every file under `policy/bundles/ops-v0.1/evidra/policy/rules/`

Replace:

- `action := input.actions[_]` → `action := defaults.actions[_]`

Also replace any indexed loops:

- `action := input.actions[i]` → `action := defaults.actions[i]`

### 3) Simplify defaults helpers to flat-only

Update:

- `policy/bundles/ops-v0.1/evidra/policy/defaults.rego`

Remove all format-agnostic K8s walking from helpers. After v2:

- `all_containers(payload)` must be only:
  - `payload.containers` + `payload.init_containers`

- `action_namespace(a)` should be only:
  - `a.payload.namespace` (optionally fallback to `a.target.namespace` if you keep it)

No `spec.*`, no `metadata.*`, no `securityContext` in defaults after v2.

---

## Tests (must add/adjust)

### OPA unit tests: canonicalizer (NEW)

Add:

- `policy/bundles/ops-v0.1/tests/canonicalize_test.rego`

Required tests:

1) Flat passthrough (semantic)
2) Deployment-like manifest extraction
3) Pod manifest extraction
4) CronJob extraction
5) Casing normalization: `securityContext` → `security_context`
6) End-to-end: native privileged container must deny with the correct rule id  
   (and must **not** deny with `ops.insufficient_context`)

### Existing tests

- Update any tests that reference `input.actions` inside decision/rules expectations only if they broke due to the boundary switch.
- Most tests should continue working because `defaults.actions` reads from `input.actions` and normalizes it.

### CI lint (recommended)

Add a simple guard to prevent drift:

- fail if any policy file references `input.actions` (except canonicalize.rego itself)

---

## Definition of Done

- `opa test policy/bundles/ops-v0.1/ -v` passes
- Native K8s manifests (Deployment/Pod/CronJob) are evaluated correctly
- `ops.insufficient_context` occurs only when data is genuinely missing
- No rules/decision reference `input.actions`
- No helpers outside canonicalize.rego walk K8s nested paths

---

## Implementation Tasks

### Step 1 — Canonicalizer module
- [ ] Create `canonicalize.rego` under `evidra.policy.defaults`
- [ ] Implement K8s shape detection: flat / workload template / pod / cronjob
- [ ] Implement projection to flat schema
- [ ] Implement container `securityContext` casing normalization

### Step 2 — Switch policy boundary
- [ ] Update `decision.rego` to read `defaults.actions`
- [ ] Update **all** rules to read `defaults.actions`
- [ ] Add CI check to forbid `input.actions` references outside canonicalize.rego

### Step 3 — Simplify helpers (remove Variant B)
- [ ] Remove multi-format K8s walking helpers from `defaults.rego`
- [ ] Keep helpers flat-only (concat containers, namespace lookup, etc.)

### Step 4 — Tests
- [ ] Add `canonicalize_test.rego` unit tests for shapes + casing
- [ ] Add an end-to-end rego test: native privileged Deployment denies with correct rule id
- [ ] Add CronJob native test (regression guard)

### Step 5 — Verification
- [ ] Run `opa test` locally and in CI
- [ ] Run existing E2E suite (MCP stdio tests) and add 1 native-manifest case if the suite supports it

---
