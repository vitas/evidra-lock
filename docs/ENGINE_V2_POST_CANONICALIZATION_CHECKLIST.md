# Evidra Engine v2 — Post-Canonicalization Checklist (Acceptance + Guardrails)

## Scope

This checklist starts **after** the canonicalization boundary is implemented:

- `canonicalize.rego` exists and exports `defaults.actions`
- `decision.rego` + all rules read `defaults.actions` (not `input.actions`)
- defaults helpers are flat-only (no K8s path walking outside canonicalize.rego)

This checklist covers:
- acceptance verification with realistic agent payloads
- guardrails to prevent architectural drift
- minimal MCP follow-ups (P1)
- readiness gates for Golden Policies / Agent Kill Switch

---

## 1) Acceptance: Realistic Agent Payload Fixtures (must add)

Create a small corpus of **native K8s manifests** as they arrive from agents.
Goal: ensure the engine v2 evaluates **correct deny reasons** and avoids false `ops.insufficient_context`.

### 1.1 Fixture format

Keep fixtures as JSON inputs to policy, mirroring your `input` structure:

- `input.tool`, `input.operation`, `input.actions[0].kind`
- `input.actions[0].payload` contains the **native manifest**

Example naming:
- `tests/corpus/native/k8s_deploy_privileged.json`
- `tests/corpus/native/k8s_pod_privileged.json`
- `tests/corpus/native/k8s_cronjob_privileged.json`

### 1.2 Minimum fixture set (P0)

1) **Deployment: privileged container**
- Expect: deny includes `k8s.privileged_container`
- Expect: deny does **not** include `ops.insufficient_context`

2) **Deployment: hostPID true**
- Expect: deny includes `k8s.host_pid` (or your equivalent)
- No `ops.insufficient_context`

3) **Pod: privileged container**
- Expect: `k8s.privileged_container`

4) **CronJob: privileged container**
- Expect: `k8s.privileged_container`

5) **Workload with initContainers + securityContext**
- Expect: initContainers are processed the same way as containers (deny if applicable)

Optional (if rules exist):
- image `:latest` / unpinned tags
- hostNetwork / hostIPC
- capabilities add/drop

### 1.3 Run modes

Each fixture should be used in:

- OPA end-to-end rego test (preferred, deterministic)
- MCP stdio E2E (if easy to add one test case)

---

## 2) OPA End-to-End Regression Tests (must add)

Add an end-to-end test file:

- `policy/bundles/ops-v0.1/tests/e2e_native_manifest_test.rego`

Required assertions per case:
- `not decision.allow`
- `"expected.rule_id"` in `decision.hits`
- `"ops.insufficient_context"` not in `decision.hits`

This is the “we never regress to false context denies” guardrail.

---

## 3) Canonicalizer Contract Tests (must keep)

Retain unit tests that validate canonicalizer output shape:

- flat passthrough semantics
- Deployment-like / Pod / CronJob extraction
- `securityContext` → `security_context` casing normalization

These tests prevent accidental breakage when someone edits canonicalize.rego.

---

## 4) Architectural Guardrails (CI lint — strongly recommended)

### 4.1 Forbid `input.actions` usage in rules/decision

Fail CI if `input.actions` appears anywhere except canonicalize.rego:

- allow: `.../policy/canonicalize.rego`
- forbid: `.../policy/decision.rego`
- forbid: `.../policy/rules/*.rego`
- forbid: `.../policy/defaults.rego`

### 4.2 Forbid K8s path walking outside canonicalize.rego

Fail CI if these patterns appear outside canonicalize.rego:

- `spec.template.spec`
- `spec.jobTemplate.spec.template.spec`
- `metadata.namespace`
- `securityContext`
- `hostPID` / `hostNetwork` / `hostIPC`

Rationale:
- prevents reintroducing Variant-B style drift

### 4.3 Require “flat-only” helpers

Optionally enforce that `defaults.rego` does not contain:
- `spec.`
- `metadata.`
- `securityContext`

---

## 5) MCP Follow-up (P1, cheap, improves UX)

After canonicalization exists, MCP schema and description should be **honest**:

- payload may be **native K8s manifest or flat**
- evidra normalizes internally
- on deny: STOP and show reasons; no retry loops without changing input

### 5.1 Minimal update targets

- validate tool description (short)
- `InputSchema.params.payload.description` (one sentence)

Keep it short; do not embed long “format contracts”.

---

## 6) Readiness Gate for Golden Policies / Agent Kill Switch

Do NOT start “Golden Policies / Agent Kill Switch” work until:

- Native Deployment/Pod/CronJob fixtures pass with correct denies
- `ops.insufficient_context` fires only for genuinely missing data
- CI guardrails prevent drift

### Golden readiness criteria (must be true)

1) Canonicalizer contract is stable (tests exist and pass)
2) At least 5 native fixtures cover destructive deny paths
3) No policy reads `input.actions` directly
4) No K8s path walking exists outside canonicalize.rego

---

## 7) Next Steps After This Checklist

Once all sections above are green:

1) Add `golden.rule_ids` registry (data-only)
2) Add a “golden regression suite” (fixtures that must always remain strict)
3) Define agent-facing behavior (Agent Kill Switch UX) using MCP guidance and UI messaging

---

## Implementation Tasks

### Step 1 — Add acceptance fixtures (native)
- [ ] Create `tests/corpus/native/` (or your repo’s fixture location)
- [ ] Add native Deployment privileged fixture + expected rule id
- [ ] Add native Pod privileged fixture
- [ ] Add native CronJob privileged fixture
- [ ] Add hostPID fixture (Deployment-like)
- [ ] Add initContainers fixture

### Step 2 — Add e2e rego tests over fixtures
- [ ] Add `e2e_native_manifest_test.rego`
- [ ] Assert correct deny IDs and absence of `ops.insufficient_context`

### Step 3 — CI guardrails
- [ ] Forbid `input.actions` outside canonicalize.rego
- [ ] Forbid K8s shape/casing patterns outside canonicalize.rego
- [ ] Run `opa test` in CI

### Step 4 — MCP P1 update
- [ ] Update validate tool description to say “native or flat accepted”
- [ ] Add short “STOP on deny” instruction in tool description/instructions

### Step 5 — Golden readiness review
- [ ] Confirm readiness criteria satisfied
- [ ] Only then proceed to Golden Policies registry + Agent Kill Switch UX work

---
