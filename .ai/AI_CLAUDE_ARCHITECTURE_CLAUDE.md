# AI Architecture Review — evidra-mcp

**Date:** 2026-02-23
**Reviewer:** Claude Code (claude-sonnet-4-6)
**Scope:** Full codebase review of `/Users/vitas/git/evidra-mcp` at branch `v1-slim`
**Status:** Analysis-only — no code changes

---

## 1. Executive Summary

Evidra-MCP is a policy enforcement and audit system for AI-agent tool use. It provides two runtime surfaces — an MCP server (`evidra-mcp`) and an offline CLI (`evidra`) — both sharing a single evaluation core that runs OPA-based policy checks against incoming tool invocations and appends tamper-evident evidence records to an append-only JSONL chain. The system is in a pre-release (v0.1-slim) state, with a working enforcement pipeline, a stable policy profile (`ops-v0.1`), and a segmented evidence store.

**Strengths:** The core design is clean. A single `EvaluateScenario` entry point enforces one evaluation path for all callers, preventing bypass divergence. The evidence chain (SHA256-linked JSONL with both in-process and file-level locking) is a correct implementation of an append-only audit ledger. The OPA policy layer is well-structured with stable rule IDs, a flat rule-per-file layout, and an accompanying OPA test suite for each rule. The `enforce`/`observe` mode split correctly separates blocking from advisory evaluation without duplicating any logic.

**Critical risks:** The central orchestrator — `pkg/validate/validate.go` at 491 lines — has zero test coverage. It contains the invocation-to-scenario conversion, per-action policy dispatch, evidence recording, and result assembly. Any regression here is undetected. A secondary structural concern is that `pkg/evidence/evidence.go` concentrates 1,215 lines of store initialization, segment management, chain validation, manifest I/O, and locking into one file, making it opaque to review and fragile to modify. ~~The `.goreleaser.yaml` ldflags reference `internal/version` — a package that does not exist — meaning release binaries silently embed the fallback `0.1.0-dev` rather than the actual release version.~~ *(Finding was incorrect — ldflags already reference `pkg/version` correctly; `internal/` does not exist in this repo.)*

---

## 2. Architecture Overview

### Two-binary design

Both binaries share `pkg/validate.EvaluateScenario` as the single evaluation entry point. Neither binary re-implements policy dispatch or evidence recording.

```
┌─────────────────────────────────────────────────────────┐
│                     cmd/evidra-mcp                      │
│   MCP server (stdio transport, official Go SDK)         │
│   pkg/mcpserver.ValidateService                         │
│     → validate.EvaluateInvocation()                     │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│                     cmd/evidra                          │
│   CLI: validate <file>  |  policy sim  |  evidence ...  │
│     → validate.EvaluateFile()                           │
│     → validate.EvaluateScenario()                       │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              pkg/validate.EvaluateScenario()            │
│                                                         │
│  config.ResolvePolicyData()                             │
│  runtime.NewEvaluator()  ←─ policysource.LocalFile()    │
│  evaluateScenarioWithRuntime()  ←─ policy.Engine.Eval() │
│  evidence.Store.Append()  ←─ evlock file lock           │
└─────────────────────────────────────────────────────────┘
```

### Enforce vs Observe modes

`ModeEnforce` (default): policy allow/deny directly controls the `OK` boolean in `ValidateOutput`. A deny result blocks the caller.

`ModeObserve`: `ValidateService.Validate()` forces `OK = true` regardless of the policy decision. Policy is still fully evaluated and the decision is still recorded in evidence. This mode is set via `--observe` flag on `evidra-mcp`.

The mode flag only lives in `pkg/mcpserver` — the CLI does not expose it (CLI callers always get enforce semantics through the exit code).

---

## 3. Package Dependency Map

Fourteen packages in `pkg/`, no circular dependencies.

```
Leaf packages (no internal imports)
────────────────────────────────────
  invocation     scenario     config
  policysource   evlock       tokens
  outputlimit    version

Mid-level packages (import leaves)
────────────────────────────────────
  policy         ←── invocation
  runtime        ←── invocation, policy, policysource
  evidence       ←── evlock, invocation

Core hub
────────────────────────────────────
  validate       ←── config, evidence, invocation, runtime, scenario

Adapter layer
────────────────────────────────────
  mcpserver      ←── config, evidence, invocation, validate
```

**Import direction is strict:** leaves flow inward to hubs; no hub imports another hub. `mcpserver` is the outermost adapter and imports `validate` — not the other way around.

**Critical shared packages:** `invocation` (canonical schema used by every non-leaf), `config` (path resolution used by both binaries), `evidence` (audit write path).

---

## 4. Data Flow Analysis

### Type chain

```
invocation.ToolInvocation
        │
        │  invocationToScenario()  (pkg/validate, lines ~154–174)
        ▼
scenario.Scenario  (one Action per invocation)
        │
        │  runtime.Evaluator.EvaluateInvocation()
        ▼
policy.Decision  (from OPA: allow, risk_level, reason, reasons, hits, hints)
        │
        │  manual field copy (pkg/runtime, EvaluateInvocation)
        ▼
runtime.ScenarioDecision  (adds PolicyRef, otherwise same fields)
        │
        │  evidence store conversion (pkg/validate)
        ▼
evidence.EvidenceRecord  (canonical immutable audit record)
```

### Where type conversions happen

**`invocationToScenario()`** (validate.go): Converts a single `ToolInvocation` into a single-action `Scenario`. Key behaviours:
- `Kind` is constructed as `"{tool}.{operation}"` using a string join with dot separator.
- `ScenarioID` resolves via a three-step fallback: `inv.Context["scenario_id"]` → `inv.Params["scenario_id"]` → generated `{tool}.{operation}.{unixnano}`.
- `Target`, `Payload`, and `RiskTags` are extracted from `inv.Params` by string key (`"target"`, `"payload"`, `"risk_tags"`).
- `Source` and `Intent` are extracted from `inv.Context` by string key (`"source"`, `"intent"`).

**`policy.Decision` → `runtime.ScenarioDecision`** (runtime.go): A manual field-by-field copy that adds `PolicyRef` from the evaluator's `policyRef` string. All other fields are identical. This is the duplicate type issue described in §8.

**`ScenarioDecision` → `evidence.PolicyDecision`** (validate.go): Another manual field copy when building the `EvidenceRecord`. Adds `Advisory bool` (true when mode is observe, always false in current CLI path since CLI doesn't support observe mode).

### Data loss points

1. **`invocationToScenario`** loses any `inv.Params` keys that are not `target`, `payload`, or `risk_tags`. If a caller passes additional parameters, they are silently discarded.
2. **`policy.Decision.Hits`** (the fired rule IDs) is copied to `runtime.ScenarioDecision.Hits` and then to the `ValidateOutput.RuleIDs` field but the key rename (`Hits` → `RuleIDs`) is invisible to callers consuming the evidence record, where the field is still named `rule_ids`.
3. **Kubernetes YAML parsing in `parseYAMLKinds()`**: If `action.Payload["inline_yaml"]` is not a string (e.g. nil), it silently returns an empty slice rather than returning an error, so bad input produces a silent empty-namespace decision.

---

## 5. Policy Layer Analysis

### Profile structure

All policy lives under `policy/profiles/ops-v0.1/`. The profile is self-contained: one shim (`policy.rego`), one aggregator (`policy/decision.rego`), one defaults file (`policy/defaults.rego`), and six rule files under `policy/rules/`.

### Rules

| Rule ID | File | Type | Condition |
|---|---|---|---|
| POL-PUB-01 | deny_public_exposure.rego | deny | `publicly_exposed` flag set in payload without `approved_public` risk tag |
| POL-PROD-01 | deny_prod_without_approval.rego | deny | Namespace matches prod pattern without `change-approved` tag |
| POL-DEL-01 | deny_mass_delete.rego | deny | `destroy_count` or `resource_count` exceeds threshold in `data.json` |
| POL-KUBE-01 | deny_kube_system.rego | deny | Action targets `kube-system` namespace without `breakglass` tag |
| WARN-BREAKGLASS-01 | warn_breakglass.rego | warn | `breakglass` tag present (allowed but flagged) |
| WARN-AUTO-01 | warn_autonomous_execution.rego | warn | Actor type is `agent` and origin is `mcp` |

### Decision aggregation (decision.rego)

```
any deny fires  → allow=false, risk_level="high"
no denies, any warn fires → allow=true, risk_level="medium"
no fires → allow=true, risk_level="low"
```

`reason` is set to the first deny reason. `reasons` collects all deny and warn reasons. `hits` collects all fired rule IDs. `hints` is resolved from `data.rule_hints[ruleID]` for each hit.

### Risk tag bypass mechanics

`warn_breakglass.rego` fires whenever the `breakglass` tag is present — it is the tag mechanism itself that drives `WARN-BREAKGLASS-01`. The deny rules check for the *absence* of required tags (`approved_public`, `change-approved`) rather than the presence of an override tag, so there is no single "override all" tag. Each deny rule is independently bypassable only by its specific required tag.

### OPA test coverage

A test file exists for each of the six rules plus `decision_contract_test.rego`. Coverage gaps:

- **Boundary conditions on thresholds:** `deny_mass_delete_test.rego` tests at the threshold and above, but does not test `destroy_count = threshold - 1` (just-below boundary).
- **Multi-violation scenarios:** No test fires two deny rules simultaneously to verify that `reasons` aggregates both and `risk_level` remains `"high"`.
- **`action_namespace` fallback:** `defaults.rego` has a fallback for namespace extraction from `Payload["namespace"]`, but the tests exercise only the direct `Target["namespace"]` path.
- **Empty `hits` when nothing fires:** `decision_contract_test.rego` tests the allow-all path but does not assert `hits == []`.

---

## 6. Evidence Layer Analysis

### Append-only JSONL chain

Each `EvidenceRecord` contains a `prev_hash` pointing to the hash of the preceding record and its own `hash` computed over a canonical JSON struct (fields in stable order, excluding `hash` itself). Chain validation replays all records in order and verifies each `prev_hash` matches the preceding computed hash. The first record sets `prev_hash = ""`.

### Segmented vs legacy storage

Storage mode is detected at open time by `detectStoreMode()`:

| Condition | Mode |
|---|---|
| Path is a directory | Segmented |
| Path is a file | Legacy (single JSONL file) |
| Path doesn't exist, extension is `.log` or `.jsonl` | Legacy |
| Path doesn't exist, any other extension or no extension | Segmented (default) |

Segmented mode maintains a `manifest.json` at the store root tracking the current segment path, sealed segment paths, total record count, and the last hash. Segments are sealed when they exceed `SegmentMaxBytes` (default 5 MB). Sealed segments are considered immutable.

Legacy mode remains supported for backward compatibility with pre-v0.1 stores.

### Locking strategy

Two layers of synchronization guard concurrent appends:

1. **`var appendMu sync.Mutex`** — package-level global. Serializes all appends within a single process. This is a hidden coupling: all `Store` instances in the same process share this lock even if they write to different directories.

2. **`evlock` file lock** — platform-specific (flock on Unix; stub/unsupported on Windows). Acquired via `withStoreLock()` around the append operation. Guards against concurrent appends from different processes.

The combination is correct for the common case (one process, one store) but the global `appendMu` becomes a bottleneck if multiple stores are used concurrently, and it prevents parallel test runs that each create a separate store (they will serialize on the global mutex rather than run truly concurrently).

### Chain validation flow

`Store.Validate()` reads all records in segment order (legacy: single file; segmented: sealed segments in order, then current segment). For each record it:
1. Recomputes the expected hash from the record's canonical struct.
2. Verifies `record.Hash == expectedHash`.
3. Verifies `record.PreviousHash == prevHash` (where `prevHash` is the hash of the preceding record).
4. Advances `prevHash`.

Validation fails on first mismatch and returns the offending record index.

---

## 7. Risk Assessment by Package

| Package | Risk | Key Issues |
|---|---|---|
| `validate` | **Critical** | 491 LOC, zero tests; contains core orchestration including invocation conversion, policy dispatch, evidence recording, and result assembly. Any regression is undetected. |
| `evidence` | **High** | 1,215 LOC in one file; concentrates store init, segment management, chain validation, manifest I/O, hash computation, and locking; global `appendMu` couples all store instances in-process. |
| `mcpserver` | **Medium** | 356 LOC; tests exist but enforce/observe mode interaction with validate is not exercised in tests; `ValidateService` directly constructs `validate.Options` without abstraction. |
| `runtime` | **Medium** | Thin but contains the redundant `ScenarioDecision` type and the manual field copy from `policy.Decision`. |
| `config` | **Medium** | 68 LOC, zero tests; path resolution logic (fallback chain, env vars, legacy var names) is completely untested. |
| `scenario` | **Low** | Tests exist; schema is simple; loader handles Terraform/K8s/explicit-action formats. |
| `policy` | **Low** | Tests exist; OPA engine wrapper is minimal; rule IDs are stable. |
| `policysource` | **Low** | Tests exist; file-based loader with deterministic `PolicyRef()` hash. |
| `invocation` | **Low** | 39 LOC; canonical schema only; tests exist. |
| `evlock` | **Low** | Platform-specific; tests exist for Unix path; Windows marked unsupported. |
| `tokens`, `outputlimit`, `version` | **Low** | Utility packages; small and non-critical. |

---

## 8. Technical Debt Inventory

### ~~TD-01 · Duplicate Decision types~~ ✓ RESOLVED 2026-02-23
`PolicyRef` added to `policy.Decision`; `runtime.ScenarioDecision` deleted. `runtime.Evaluator.EvaluateInvocation` now returns `policy.Decision` directly, stamping `PolicyRef` in-place before return. Single authoritative type through the full pipeline; no manual field copy. `PolicyRef` propagation covered by `TestEvaluateInvocationSetsPolicyRef`.

### ~~TD-02 · No tests for validate.go~~ ✓ RESOLVED 2026-02-23
Two test files added. `pkg/validate/validate_internal_test.go` (`package validate`) covers unexported helpers: `splitKind` (8 table-driven cases including whitespace edge cases), `dedupeStrings` (4 subtests), `invocationToScenario` field mapping and source fallback, and `scenarioIDFromInvocation` 3-level priority chain. `pkg/validate/validate_test.go` (`package validate_test`) covers the public API: allow path, deny path (POL-PROD-01), breakglass warn path (WARN-BREAKGLASS-01), SkipEvidence flag, invalid action kind, multi-action one-deny, and invocation payload reaching OPA. All three sentinel error paths (ErrInvalidInput, ErrPolicyFailure, ErrEvidenceWrite) are also exercised in the same file.

### ~~TD-03 · Magic string keys throughout invocation/context/payload handling~~ ✓ RESOLVED 2026-02-23
Six typed constants added to `pkg/invocation`: `KeyTarget`, `KeyPayload`, `KeyRiskTags`, `KeyScenarioID`, `KeySource`, `KeyIntent`. All 11 raw string literals in `pkg/validate/validate.go` (`invocationToScenario`, `scenarioIDFromInvocation`, OPA input builder) and 3 in `pkg/invocation/invocation.go` (`ValidateStructure`) replaced with the constants. Test map literals in `invocation_test.go` updated likewise. A typo in any of these keys now causes a compilation error.

### ~~TD-04 · No PolicySource interface~~ ✓ RESOLVED 2026-02-23
`runtime.PolicySource` interface defined in `pkg/runtime` with three methods: `LoadPolicy`, `LoadData`, `PolicyRef`. `runtime.NewEvaluator` now accepts the interface; `pkg/policysource.LocalFileSource` satisfies it implicitly. `runtime` no longer imports `policysource` — the concrete dependency moved to callers (`validate.go`, tests). Error-path tests added using an inline `fakeSource` test double: `TestNewEvaluatorLoadPolicyError`, `TestNewEvaluatorLoadDataError`, `TestNewEvaluatorPolicyRefError`.

### ~~TD-05 · Global appendMu in evidence package~~ ✓ RESOLVED 2026-02-23
`var appendMu sync.Mutex` removed from `evidence.go`. `mu sync.Mutex` added to `Store` struct in `store.go`. `Store.Append` and `Store.ValidateChain` now acquire `s.mu` before calling the underlying path-level functions; `appendAtPathUnlocked` and `validateChainAtPathUnlocked` are now correctly lock-free. Each store instance owns its own mutex — concurrent appends to different stores no longer serialize. Verified with `-race`: `TestConcurrentAppendsSameStore` (8 goroutines, same store, chain validates) and `TestConcurrentAppendsDifferentStores` (two stores at independent paths, fully parallel).

### ~~TD-06 · evidence.go is 1,215 lines~~ ✓ RESOLVED 2026-02-23
Split into 8 focused files. `evidence.go` reduced to 200 lines (public API + `detectStoreMode` + mode-dispatch routing). New files: `types.go` (167 lines — all types, constants, sentinel errors, `StoreError`, `ChainValidationError`), `hash.go` (31 lines — `ComputeHash`), `io.go` (55 lines — `streamFileRecords`, `appendRecordLine`), `lock.go` (75 lines — `withStoreLock`, `lockRootForPath`, `lockTimeoutFromEnv`, `segmentMaxBytesFromEnv`), `manifest.go` (124 lines — `ManifestPath`, `LoadManifest`, `loadOrInitManifest`, `writeManifestAtomic`), `segment.go` (344 lines — full segment lifecycle: append, validate chain, stream, naming/sorting helpers, `SegmentFiles`), `legacy.go` (141 lines — legacy mode append/validate/stream/readLast), `forwarder.go` (141 lines — forwarder state and cursor resolution). `store.go` and `resource_links.go` unchanged. All tests pass with `-race`.

### ~~TD-07 · Fragile invocationToScenario — silent data loss~~ ✓ RESOLVED 2026-02-24
Strict validation of unknown `Params` and `Context` keys added to `ValidateStructure()` in `pkg/invocation`. Two package-level maps (`allowedParamKeys`, `allowedContextKeys`) define the canonical key sets derived from the `Key*` constants. A `rejectUnknownKeys` helper returns a descriptive error for any key not in the allowed set. The redundant `ValidateStructure()` call inside `runtime.EvaluateInvocation` was removed — validation occurs at the boundary entry points (`pkg/validate`, `cmd/evidra/policy_sim_cmd`), and the runtime evaluator receives internally-constructed invocations with different key schemas. Three tests added: `TestValidateStructure_UnknownParamsKeyFails`, `TestValidateStructure_UnknownContextKeyFails`, `TestValidateStructure_AllKnownKeysPass`. All tests pass with `-race`.

### ~~TD-08 · Inconsistent error handling~~ ✓ RESOLVED 2026-02-23
Three sentinel errors added to `pkg/validate`: `ErrInvalidInput`, `ErrPolicyFailure`, `ErrEvidenceWrite`. All error return points in `EvaluateInvocation` and `EvaluateScenario` wrap with the appropriate sentinel via `fmt.Errorf("%w: %w", sentinel, err)`, making `errors.Is()` work for all callers. In `pkg/mcpserver`: error code strings replaced with named constants (`ErrCodeInvalidInput`, `ErrCodePolicyFailure`, `ErrCodeEvidenceWrite`, `ErrCodeChainInvalid`, `ErrCodeNotFound`, `ErrCodeInternalError`); `validateErrCode()` helper maps validate errors to codes via `errors.Is()` instead of a single `"internal_error"` catch-all. First test file for `pkg/validate` added (`validate_test.go`): covers `ErrInvalidInput` (empty invocation), `ErrPolicyFailure` (nonexistent policy path), and `ErrEvidenceWrite` (ENOTDIR on store init). Two mcpserver tests added for code mapping: `TestValidateServiceBadPolicyReturnsCode` and `TestValidateServiceInvalidInputReturnsCode`.

---

## 9. Design vs. Implementation Gaps

The following items appear in `ai/AI_DECISIONS.md` as decided but are not present in the current codebase. These are backlog items, not bugs.

| Designed item | Decision date | Status |
|---|---|---|
| `policy/reason_codes.rego` — standardized reason code vocabulary | 2026-02-20 | Not created |
| Policy templates (`dev_safe`, `regulated_dev`, `ci_agent`) under `policy/templates/` | 2026-02-20 | Not created |
| `spec/POLICY_TEMPLATES.md` — policy template usage doc | 2026-02-20 | Not created |
| `cmd/evidra-policy-sim` — standalone offline policy simulator | 2026-02-20 | Not created |
| `--guarded` flag (referenced in design context) | — | Not implemented |
| `core.PolicySource`, `core.PolicyEngine`, `core.EvidenceStore` interfaces in a `core` package | 2026-02-20 | Not created; interfaces do not exist |
| `LoadFromFiles(policyPath, dataPaths)` on policy loader | 2026-02-20 | Not present in current `pkg/policy` |
| `policy_ref` in every evidence record (per AI_DECISIONS) | 2026-02-20 | Present in `runtime.ScenarioDecision` and passed through; appears in `evidence.PolicyDecision` as part of the record — **this one IS implemented** |

The unimplemented items from 2026-02-20 (reason codes, templates, policy-sim binary, guarded flag, core interfaces) represent a significant gap between the documented design intent and the current code. If the `v1-slim` branch is intended to be the minimal shippable surface, these items should either be moved to a future milestone or the AI_DECISIONS.md entries should be annotated as deferred.

---

## 10. Recommendations (Prioritized)

### P0 · Critical

**Add tests for `pkg/validate`**
Write table-driven tests covering: (1) `invocationToScenario` field mapping, (2) deny decision propagates `Pass=false`, (3) warn decision propagates `Pass=true` with `RiskLevel="medium"`, (4) `SkipEvidence=true` skips evidence write, (5) evidence write failure returns error. A fake `PolicySource` and fake `evidence.Store` (see TD-04 and TD-05) will be necessary to make this unit-testable without filesystem I/O.

### P1 · High

~~**Merge `policy.Decision` and `runtime.ScenarioDecision` (TD-01)**~~ ✓ DONE 2026-02-23

~~**Define a `PolicySource` interface (TD-04)**~~ ✓ DONE 2026-02-23

### P2 · Medium

**Replace magic string keys with typed constants (TD-03)**
Add a `const` block to `pkg/invocation` (e.g. `KeyTarget = "target"`, `KeyPayload = "payload"`, etc.). Use these constants in `invocationToScenario`, OPA input building, and any CLI or test code that constructs `ToolInvocation.Params`.

**Split `evidence.go` into focused files (TD-06)**
Split the 1,215-line file into `store.go`, `segment.go`, `chain.go`, `manifest.go`. ~~`appendMu` relocation~~ already done as part of TD-05.

**Add tests for `pkg/config` (TD-08 adjacent)**
`config.ResolvePolicyData` and `config.ResolveEvidenceDir` have a multi-step fallback chain with env vars, legacy var names, and repo-relative defaults. This logic is untested. Write tests using `t.Setenv` to cover each fallback branch.

### P3 · Low

~~**Fix goreleaser ldflags (TD-09)**~~ ✗ NOT NEEDED — ldflags already reference `pkg/version`; finding was a review error.

**Resolve or defer AI_DECISIONS backlog (§9)**
Annotate each unimplemented 2026-02-20 decision in `ai/AI_DECISIONS.md` with a `[DEFERRED]` marker and a milestone reference, or delete the entries if they are no longer planned. Undifferentiated backlog in governance docs makes it impossible to distinguish "decided and done" from "decided but not started".

---

*Review generated by Claude Code (claude-sonnet-4-6) on 2026-02-23. Based on static code analysis only — no runtime testing performed.*
