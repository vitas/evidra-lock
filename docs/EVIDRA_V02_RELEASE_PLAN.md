# Evidra v0.2 — Release Implementation Plan

**Source:** Architecture Review findings (C1–C4, S1–S3, I1–I6, P1–P4, M1–M5)
**Target:** Claude Code agent execution — each section is a self-contained task
**Prerequisite:** v0.1 release with C1–C4 and S1–S2 documentation fixes already shipped

---

## How to Use This Document

Each task below is structured as an instruction block for Claude Code. Tasks are grouped into phases and ordered by dependency. Within each phase, tasks are independent and can be executed in parallel.

**Convention:**
- `[FILES]` — files to create or modify
- `[TEST]` — verification command
- `[DONE WHEN]` — acceptance criteria

---

## Phase 0: v0.1 Release Prerequisites (Ship First)

> These must be completed before starting v0.2 work. If not yet done, execute these first.

### T-0.1: Cache OPA Evaluator (C1)

**Problem:** `pkg/validate/EvaluateScenario()` creates a new `runtime.Evaluator` on every call. OPA query compilation costs 50–200ms per invocation.

**Implementation:**
1. In `pkg/validate/validate.go`, add a package-level evaluator cache using `sync.Once` or a simple mutex-protected map keyed by bundle path.
2. `EvaluateScenario` should check the cache first. If the bundle path + policy ref match, reuse the evaluator.
3. The MCP server and CLI should benefit automatically since they call `EvaluateInvocation` → `EvaluateScenario`.

```
[FILES]
  pkg/validate/validate.go        — add evaluator cache
  pkg/validate/validate_test.go   — add test: two sequential evaluations reuse the same engine

[TEST]  go test ./pkg/validate/ -run TestEvaluatorCacheReuse -v
[DONE WHEN] OPA PreparedEvalQuery is compiled once per bundle, not per call.
```

### T-0.2: ULID for Evidence Event IDs (C2)

**Problem:** `fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())` is not unique under concurrent access.

**Implementation:**
1. In `pkg/validate/validate.go`, replace the event ID generation:
   ```go
   // Before:
   evidenceID = fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())
   // After:
   evidenceID = "evt-" + ulid.Make().String()
   ```
2. Add `"github.com/oklog/ulid/v2"` import (already in go.mod).

```
[FILES]
  pkg/validate/validate.go   — replace evidenceID generation
  
[TEST]  go test ./pkg/validate/ -v
[DONE WHEN] Event IDs use ULID format: evt-01JXXXXXXXXXXXXXXXXXXXXXXXXX
```

### T-0.3: Canonical Record Drift Test (C3)

**Problem:** `canonicalEvidenceRecord` in `pkg/evidence/types.go` may drift from `EvidenceRecord` — fields added to one but not the other won't be covered by the hash chain.

**Implementation:**
1. Add a test in `pkg/evidence/hash_test.go` that uses `reflect` to compare the field names of `EvidenceRecord` and `canonicalEvidenceRecord`.
2. The test should fail if `EvidenceRecord` has a field not present in `canonicalEvidenceRecord` (excluding `Hash` which is intentionally omitted).

```go
func TestCanonicalRecordFieldParity(t *testing.T) {
    full := reflect.TypeOf(EvidenceRecord{})
    canon := reflect.TypeOf(canonicalEvidenceRecord{})
    canonFields := map[string]bool{}
    for i := 0; i < canon.NumField(); i++ {
        canonFields[canon.Field(i).Name] = true
    }
    excluded := map[string]bool{"Hash": true}
    for i := 0; i < full.NumField(); i++ {
        name := full.Field(i).Name
        if excluded[name] { continue }
        if !canonFields[name] {
            t.Errorf("EvidenceRecord.%s is not in canonicalEvidenceRecord — hash chain won't cover it", name)
        }
    }
}
```

```
[FILES]
  pkg/evidence/hash_test.go   — add TestCanonicalRecordFieldParity

[TEST]  go test ./pkg/evidence/ -run TestCanonicalRecordFieldParity -v
[DONE WHEN] Test passes. Adding a field to EvidenceRecord without updating canonical will fail CI.
```

### T-0.4: Migration Tracking Table (C4)

**Problem:** `internal/db/db.go` runs all SQL migrations on every startup. No tracking of which migrations have already been applied.

**Implementation:**
1. In `runMigrations()`, first ensure a tracking table exists:
   ```sql
   CREATE TABLE IF NOT EXISTS schema_migrations (
       filename TEXT PRIMARY KEY,
       applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   ```
2. Before executing each migration file, check if `filename` exists in `schema_migrations`.
3. After successful execution, insert the filename.
4. Wrap the check + execute + insert in a transaction per migration.

```
[FILES]
  internal/db/db.go           — add migration tracking logic
  internal/db/db_test.go      — add test: re-running migrations is a no-op

[TEST]  go test ./internal/db/ -v
[DONE WHEN] Second call to runMigrations() executes zero SQL statements.
```

### T-0.5: Document Trust Boundaries in SECURITY_MODEL.md (S1 + S2)

**Implementation:**
Add two new sections to `docs/SECURITY_MODEL.md`:

1. **"Runtime Trust Boundary"** section after "Known Limitations":
   - Evidra assumes host integrity
   - No binary integrity validation
   - Policy bundle integrity guaranteed at build time only
   - Evidence integrity assumes uncompromised writer process
   - Signing key confidentiality depends on OS-level process isolation

2. **"Blast Radius by Compromise Level"** — table with four rows:
   - API key leak → decision spoofing (new records only), no history rewrite
   - Database access → metadata exposure, no plaintext key recovery
   - Evidence FS access → rewrite possible (rewind attack), no API evidence forgery
   - Host compromise → full trust collapse

3. **"Explicit Non-Goals (v0.1)"** section:
   - Host compromise detection
   - Runtime binary integrity verification
   - Distributed consensus on evidence
   - Byzantine fault tolerance
   - Multi-region consistency
   - Post-incident forensic chain-of-custody guarantees

4. **"Evidence Rewind Attack"** note in Evidence Integrity section:
   - Hash chain protects against modification, not truncation
   - External anchoring planned for v1.0

5. Add one line to the introduction:
   > Evidra is a preventive control, not a post-incident forensic system.

```
[FILES]
  docs/SECURITY_MODEL.md

[TEST]  Manual review — grep for "Runtime Trust Boundary", "Non-Goals", "Rewind"
[DONE WHEN] All five additions present. No existing content removed.
```

---

## Phase 1: Runtime Hardening

### T-1.1: Typed Validation Errors (I1)

**Problem:** `isValidationError()` in `internal/api/validate_handler.go` uses string matching on error messages.

**Implementation:**
1. In `pkg/invocation/invocation.go`, define:
   ```go
   type ValidationError struct {
       Field   string
       Code    string
       Message string
   }
   func (e *ValidationError) Error() string { return e.Message }
   ```
2. In `ValidateStructure()`, return `&ValidationError{Field: "actor.type", Code: "required", Message: "actor.type is required"}` instead of `errors.New(...)`.
3. In `rejectUnknownKeys()`, return `&ValidationError{Field: key, Code: "unknown_key", ...}`.
4. In `internal/api/validate_handler.go`, replace `isValidationError()`:
   ```go
   var ve *invocation.ValidationError
   if errors.As(err, &ve) {
       writeError(w, http.StatusBadRequest, ve.Error())
       return
   }
   ```
5. Update existing tests to assert on error type, not message text.

```
[FILES]
  pkg/invocation/invocation.go         — add ValidationError type, update returns
  pkg/invocation/invocation_test.go    — update assertions
  internal/api/validate_handler.go     — replace isValidationError()
  internal/api/api_test.go             — update assertions

[TEST]  go test ./pkg/invocation/ ./internal/api/ -v
[DONE WHEN] No string matching on error messages. errors.As() used everywhere.
```

### T-1.2: Immutable Input in API Handler (I2)

**Problem:** `buildCanonicalAction()` mutates `inv.Params` in-place, silently dropping unknown fields.

**Implementation:**
1. In `validate_handler.go`, clone `inv` before canonicalization:
   ```go
   canonInv := inv // shallow copy of struct
   if canonInv.Params != nil {
       if _, hasAction := canonInv.Params["action"]; !hasAction {
           canonInv.Params = buildCanonicalParams(inv.Params, inv.Tool, inv.Operation)
       }
   }
   ```
2. Rename `buildCanonicalAction` to `buildCanonicalParams`, return a new map instead of mutating.
3. Pass `canonInv` to `eng.Evaluate()`, keep original `inv` for evidence/logging.
4. If unknown param keys are present, add a `warnings` field to the response (non-breaking addition).

```
[FILES]
  internal/api/validate_handler.go   — clone before mutation, rename function
  internal/api/api_test.go           — add test: original params preserved after validate

[TEST]  go test ./internal/api/ -v
[DONE WHEN] Original inv.Params is never modified. Unknown fields logged or warned.
```

### T-1.3: MCP Input Depth and Complexity Guards (S3)

**Problem:** MCP stdio path has no limits on JSON depth, map size, or OPA evaluation time.

**Implementation:**
1. Create `pkg/inputguard/guard.go`:
   ```go
   const MaxJSONDepth = 32
   const MaxMapKeys = 1000
   
   func CheckDepth(v interface{}, maxDepth int) error { ... }
   func CheckMapSize(v interface{}, maxKeys int) error { ... }
   ```
2. In `pkg/mcpserver/server.go`, after receiving the `ToolInvocation`, call:
   ```go
   if err := inputguard.CheckDepth(inv.Params, inputguard.MaxJSONDepth); err != nil { ... }
   if err := inputguard.CheckMapSize(inv.Params, inputguard.MaxMapKeys); err != nil { ... }
   ```
3. In `pkg/policy/policy.go`, replace `context.Background()` with a timeout context:
   ```go
   ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
   defer cancel()
   results, err := e.query.Eval(ctx, rego.EvalInput(input))
   ```
4. Propagate context from MCP handler through to OPA evaluation.

```
[FILES]
  pkg/inputguard/guard.go        — new package
  pkg/inputguard/guard_test.go   — depth and size tests
  pkg/mcpserver/server.go        — add guards before validation
  pkg/policy/policy.go           — add context timeout to Eval

[TEST]  go test ./pkg/inputguard/ ./pkg/policy/ -v
[DONE WHEN] Deeply nested input rejected. OPA eval times out after 5s.
```

### T-1.4: API Rate Limiting for POST /v1/keys (I3)

**Problem:** Documented "3 keys/hour/IP" rate limit is not implemented.

**Implementation:**
1. Create `internal/api/ratelimit.go` — in-memory token bucket per IP:
   ```go
   type RateLimiter struct {
       mu      sync.Mutex
       buckets map[string]*bucket
       rate    int           // tokens per window
       window  time.Duration
   }
   ```
2. Apply as middleware to `POST /v1/keys` only:
   ```go
   mux.Handle("POST /v1/keys", rateLimiter.Wrap(handleKeys(cfg.Store, cfg.InviteSecret)))
   ```
3. On limit exceeded, return `429 Too Many Requests` with `Retry-After` header.
4. Use `X-Forwarded-For` / `X-Real-IP` with fallback to `RemoteAddr`.
5. Add periodic cleanup of stale buckets (goroutine with ticker).

```
[FILES]
  internal/api/ratelimit.go        — new file
  internal/api/ratelimit_test.go   — test: 4th request within window returns 429
  internal/api/router.go           — wire rate limiter to /v1/keys

[TEST]  go test ./internal/api/ -run TestRateLimit -v
[DONE WHEN] 4th POST /v1/keys in 1 hour returns 429. Stale buckets cleaned up.
```

### T-1.5: API Hardening — Content-Type and Timeouts (I6)

**Problem:** No Content-Type enforcement on POST endpoints. `ReadTimeout` and `WriteTimeout` unset.

**Implementation:**
1. Add `contentTypeMiddleware` in `internal/api/middleware.go`:
   ```go
   func contentTypeMiddleware(next http.Handler) http.Handler {
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           if r.Method == http.MethodPost {
               ct := r.Header.Get("Content-Type")
               if !strings.HasPrefix(ct, "application/json") {
                   writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
                   return
               }
           }
           next.ServeHTTP(w, r)
       })
   }
   ```
2. Add to middleware stack in `NewRouter()`.
3. In `cmd/evidra-api/main.go`, set server timeouts:
   ```go
   srv := &http.Server{
       ReadTimeout:       30 * time.Second,
       WriteTimeout:      30 * time.Second,
       ReadHeaderTimeout: 10 * time.Second,
       IdleTimeout:       60 * time.Second,
   }
   ```

```
[FILES]
  internal/api/middleware.go     — add contentTypeMiddleware
  internal/api/router.go        — wire content-type middleware
  internal/api/api_test.go      — test: POST with text/plain returns 415
  cmd/evidra-api/main.go        — set ReadTimeout + WriteTimeout

[TEST]  go test ./internal/api/ -v
[DONE WHEN] POST with wrong Content-Type returns 415. All four timeouts set.
```

### T-1.6: Graceful TouchKey Shutdown (I4)

**Problem:** `TouchKey` fires goroutines with `context.Background()` that aren't drained on shutdown.

**Implementation:**
1. Add a `Close()` method to `KeyStore` that signals pending goroutines to stop.
2. Use a `sync.WaitGroup` to track in-flight touch operations.
3. In `cmd/evidra-api/main.go`, call `ks.Close()` during graceful shutdown before `pool.Close()`.

```go
type KeyStore struct {
    pool *pgxpool.Pool
    wg   sync.WaitGroup
}

func (s *KeyStore) TouchKey(keyID string) {
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        // ... existing logic
    }()
}

func (s *KeyStore) Close() { s.wg.Wait() }
```

```
[FILES]
  internal/store/keys.go      — add WaitGroup + Close()
  cmd/evidra-api/main.go      — call ks.Close() in shutdown path

[TEST]  go test ./internal/store/ -v
[DONE WHEN] Server shutdown waits for pending touch operations.
```

---

## Phase 2: Policy & Evidence

### T-2.1: OPA Canary Rule + Decision Validation (P1)

**Problem:** OPA is fail-open (`default allow := true`). A partially loaded bundle can silently allow everything.

**Implementation:**
1. Add canary rule `policy/bundles/ops-v0.1/evidra/policy/rules/sys_bundle_loaded.rego`:
   ```rego
   package evidra.policy
   warn["sys.bundle_loaded"] = "bundle integrity canary" if { true }
   ```
2. In `pkg/policy/policy.go`, after `Evaluate()`, verify the canary:
   ```go
   if decision.Allow && !containsHit(decision.Hits, "sys.bundle_loaded") {
       return Decision{Allow: false, RiskLevel: "high", Reason: "policy_bundle_incomplete"}, 
              errors.New("canary rule sys.bundle_loaded not found in decision — bundle may be partially loaded")
   }
   ```
3. Add `decision_schema_version` field to the Rego decision object:
   ```rego
   decision := {
       "schema_version": 1,
       "allow": allow,
       ...
   }
   ```
4. In Go, verify `schema_version == 1`; reject unknown versions.

```
[FILES]
  policy/bundles/ops-v0.1/evidra/policy/rules/sys_bundle_loaded.rego   — new canary
  policy/bundles/ops-v0.1/evidra/policy/decision.rego                  — add schema_version
  policy/bundles/ops-v0.1/tests/sys_bundle_loaded_test.rego            — test canary fires
  pkg/policy/policy.go                                                  — verify canary + schema version
  pkg/policy/policy_test.go                                             — test partial bundle detection

[TEST]  go test ./pkg/policy/ -v && opa test policy/bundles/ops-v0.1/ -v
[DONE WHEN] Partially loaded bundle produces deny. Schema version mismatch produces deny.
```

### T-2.2: Policy Contract Golden Tests (P3)

**Problem:** Example fixtures in `examples/` aren't wired as automated regression tests.

**Implementation:**
1. Create sidecar files for each fixture:
   ```
   examples/demo/kubernetes_kube_system_delete.json
   examples/demo/kubernetes_kube_system_delete.expected.json  ← NEW
   ```
   Expected format:
   ```json
   {"allow": false, "rule_ids": ["k8s.protected_namespace"], "risk_level": "high"}
   ```
2. Add `pkg/validate/golden_test.go`:
   ```go
   func TestGoldenScenarios(t *testing.T) {
       fixtures, _ := filepath.Glob("../../examples/demo/*.json")
       for _, f := range fixtures {
           if strings.HasSuffix(f, ".expected.json") { continue }
           expected := f[:len(f)-5] + ".expected.json"
           // load, evaluate, compare allow + rule_ids + risk_level
       }
   }
   ```
3. Add `.expected.json` files for all existing fixtures in `examples/demo/` and `examples/`.

```
[FILES]
  examples/demo/*.expected.json                — new sidecar files
  examples/*.expected.json                     — new sidecar files
  pkg/validate/golden_test.go                  — new test file

[TEST]  go test ./pkg/validate/ -run TestGoldenScenarios -v
[DONE WHEN] All fixtures have expected files. CI fails if policy change alters decision.
```

### T-2.3: Evidence Progress and Incremental Verify (I5)

**Problem:** `evidra evidence verify` is O(N) with no progress output. No way to do quick health-check.

**Implementation:**
1. In `cmd/evidra/evidence_cmd.go`, add `--last N` flag to verify:
   ```
   evidra evidence verify --last 100
   ```
2. Add `--progress` flag that prints record count every 1000 records.
3. Implement `VerifyLastN(path string, n int) error` in `pkg/evidence/`:
   - Read manifest to get total records
   - Seek to the last N records (use segment boundaries)
   - Verify hash chain from record (total-N) to end
4. For `evidra evidence export`, add progress to stderr.

```
[FILES]
  pkg/evidence/evidence.go          — add VerifyLastN()
  pkg/evidence/evidence_test.go     — test incremental verify
  cmd/evidra/evidence_cmd.go        — add --last and --progress flags

[TEST]  go test ./pkg/evidence/ -run TestVerifyLastN -v
[DONE WHEN] `evidra evidence verify --last 100` completes in <1s on large stores.
```

### T-2.4: Rule Authoring Checklist + Template (P4)

**Problem:** No step-by-step guide for adding custom OPA rules.

**Implementation:**
1. Create `docs/RULE_AUTHORING.md` with checklist:
   - Step 1: Create `deny_<name>.rego` in `evidra/policy/rules/`
   - Step 2: Add hints in `evidra/data/rule_hints/data.json`
   - Step 3: Add entry to `docs/POLICY_CATALOG.md`
   - Step 4: Add test `tests/deny_<name>_test.rego`
   - Step 5: Add scenario fixture in `examples/` with `.expected.json`
   - Step 6: Gate on `profile_includes_ops` if ops-layer rule
2. Create `policy/bundles/ops-v0.1/evidra/policy/rules/_template.rego`:
   ```rego
   # Rule: <rule_id>
   # Severity: deny | warn
   # Description: <what this catches>
   # Catalog: docs/POLICY_CATALOG.md
   package evidra.policy
   import data.evidra.policy.defaults as defaults
   
   deny["<domain>.<rule_name>"] = "<human-readable message>" if {
       defaults.profile_includes_ops
       action := input.actions[_]
       action.kind == "<tool>.<operation>"
       # ... rule logic ...
   }
   ```
3. Add link from `CONTRIBUTING.md` to `docs/RULE_AUTHORING.md`.

```
[FILES]
  docs/RULE_AUTHORING.md                                              — new guide
  policy/bundles/ops-v0.1/evidra/policy/rules/_template.rego          — new template
  CONTRIBUTING.md                                                      — add link

[TEST]  Manual review
[DONE WHEN] A new contributor can add a rule by following the checklist without reading source code.
```

---

## Phase 3: Moderate Fixes

### T-3.1: Deep Copy for Nested Maps (M1)

Replace `copyMap()` in `pkg/validate/validate.go` with a recursive deep copy:
```go
func deepCopyMap(src map[string]interface{}) map[string]interface{} {
    out := make(map[string]interface{}, len(src))
    for k, v := range src {
        switch val := v.(type) {
        case map[string]interface{}:
            out[k] = deepCopyMap(val)
        case []interface{}:
            out[k] = deepCopySlice(val)
        default:
            out[k] = v
        }
    }
    return out
}
```

```
[FILES]  pkg/validate/validate.go, pkg/validate/validate_test.go
[TEST]   go test ./pkg/validate/ -run TestDeepCopy -v
```

### T-3.2: Unified Logging (M3)

Replace `log.New(stderr, ...)` in `cmd/evidra-mcp/main.go` with `slog` to match the API binary.

```
[FILES]  cmd/evidra-mcp/main.go
[TEST]   Build and run — verify structured log output on stderr
```

### T-3.3: CORS Headers (M2)

Add basic CORS middleware in `internal/api/middleware.go` for the API server. Default: allow same-origin only. Configurable via `EVIDRA_CORS_ORIGINS` env var.

```
[FILES]  internal/api/middleware.go, internal/api/router.go
[TEST]   go test ./internal/api/ -run TestCORS -v
```

### T-3.4: API Reference (OpenAPI)

Generate an OpenAPI 3.0 spec for the v1 API surface. Include request/response schemas for all endpoints. Serve at `GET /v1/openapi.json`.

```
[FILES]  docs/openapi.yaml (or .json), internal/api/router.go
[TEST]   Manual — validate with spectral or swagger-cli
```

---

## Phase 4: Release

### T-4.1: CHANGELOG Update

Add `## [v0.2.0]` section to `CHANGELOG.md` covering all changes from Phases 1–3.

### T-4.2: Version Bump

Update `VERSION` file to `0.2.0`. Update version constants in `pkg/version/`.

### T-4.3: CI Golden Gate

Ensure CI runs:
```bash
go test ./...
opa test policy/bundles/ops-v0.1/ -v
```
Both must pass before tag.

### T-4.4: Docker Image Rebuild

Rebuild and tag `ghcr.io/vitas/evidra-mcp:v0.2.0` and `ghcr.io/vitas/evidra-api:v0.2.0`.

### T-4.5: Release Checklist Validation

Run `docs/RELEASE_CHECKLIST.md` items. Verify `verify_p0.sh` still passes (backward compatibility).

---

## Task Dependency Graph

```
Phase 0 (v0.1 prerequisites)
  T-0.1 ──┐
  T-0.2 ──┤
  T-0.3 ──┼── All must pass before Phase 1
  T-0.4 ──┤
  T-0.5 ──┘

Phase 1 (runtime hardening) — all independent
  T-1.1 (typed errors)
  T-1.2 (immutable input)
  T-1.3 (input guards) ← depends on T-0.1 (cached evaluator for timeout context)
  T-1.4 (rate limiting)
  T-1.5 (API hardening)
  T-1.6 (graceful shutdown)

Phase 2 (policy & evidence) — partial dependencies
  T-2.1 (canary rule) ← independent
  T-2.2 (golden tests) ← depends on T-2.1 (canary changes decision output)
  T-2.3 (evidence progress) ← independent
  T-2.4 (rule authoring) ← depends on T-2.2 (checklist references golden tests)

Phase 3 (moderate fixes) — all independent

Phase 4 (release) — depends on all above
```

---

## Acceptance Criteria for v0.2 Release

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | All v0.1 prerequisites shipped (C1–C4, S1–S2) | `verify_p0.sh` passes |
| 2 | No string matching on error types | `grep -r "strings.Contains.*is required" internal/` returns nothing |
| 3 | API key rate limit enforced | `curl` test: 4th POST /v1/keys in window → 429 |
| 4 | MCP rejects depth > 32 | Unit test passes |
| 5 | OPA canary rule fires on every evaluation | `go test ./pkg/policy/ -run TestCanary` |
| 6 | Golden tests cover all example fixtures | `go test ./pkg/validate/ -run TestGoldenScenarios` |
| 7 | `evidra evidence verify --last 100` works | Manual test on evidence store with 1000+ records |
| 8 | SECURITY_MODEL.md has trust boundaries, blast radius, non-goals | Manual review |
| 9 | RULE_AUTHORING.md exists with template | Manual review |
| 10 | All timeouts set on HTTP server | Grep for `ReadTimeout`, `WriteTimeout` in main.go |
| 11 | `go test ./...` passes | CI |
| 12 | `opa test policy/bundles/ops-v0.1/ -v` passes | CI |

---

## Estimated Effort

| Phase | Tasks | Estimated Total |
|-------|-------|----------------|
| Phase 0 | 5 tasks | 1–2 days |
| Phase 1 | 6 tasks | 3–4 days |
| Phase 2 | 4 tasks | 2–3 days |
| Phase 3 | 4 tasks | 1–2 days |
| Phase 4 | 5 tasks | 0.5 day |
| **Total** | **24 tasks** | **~8–12 days** |
