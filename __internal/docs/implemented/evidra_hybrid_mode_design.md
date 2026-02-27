# Evidra — Hybrid Mode Refactoring Design

**Date:** 2026-02-26
**Status:** Ready for implementation
**Scope:** CLI + MCP transition from offline-only to API-first hybrid with offline fallback
**Module:** `samebits.com/evidra` (existing repo)

---

## 1. Problem Statement

Currently Evidra has **two disconnected evaluation paths**:

- **Local path** (CLI + MCP): loads OPA bundle locally, evaluates in-process, writes evidence to `~/.evidra/evidence` JSONL.
- **API path** (`evidra-api`): HTTP server with PostgreSQL, tenant isolation, skills, server-side evidence signing.

Both share `pkg/validate.EvaluateScenario()` as their evaluation core, but they have different evidence stores, different policy sources, and different tenant models. A developer using the CLI and a CI pipeline using the API can get different policy decisions if their bundles diverge.

**Goal:** Make CLI and MCP API-first — when `EVIDRA_URL` is configured, they delegate all evaluation to the API server. Local OPA becomes a fallback for offline/air-gapped scenarios. The API is the single source of truth for policy and evidence.

---

## 2. Design Decisions

| # | Decision | Rationale |
|---|---|---|
| 1 | CLI and MCP use HTTP client when `EVIDRA_URL` is set | API is source of truth. Consistent policy across all clients |
| 2 | Fallback default: **fail closed** | Silent fallback to stale local bundle could allow actions the server would deny |
| 3 | `EVIDRA_FALLBACK=offline` opts into local evaluation when API unreachable | Air-gapped and unstable-network environments need this escape hatch |
| 4 | MCP in online mode proxies through `evidra-api` as sidecar on localhost | MCP stays focused on stdio transport. API handles auth, policy, evidence, skills. Adding features to API gives MCP them for free |
| 5 | Evidence from fallback mode marked `"source": "local-fallback"` | Auditors can distinguish server-authoritative vs degraded-mode decisions |
| 6 | `evidra evidence sync` (local → API) is Phase 2, not in this refactor | Sync requires API evidence ingestion endpoint that doesn't exist yet |
| 7 | `--offline` flag forces offline mode even when `EVIDRA_URL` is set | Developer convenience: quick local testing without unsetting env vars |
| 8 | New package `pkg/client` for HTTP client | Clean separation. No HTTP in `pkg/validate` (stays pure). Client is thin wrapper around `POST /v1/validate` |

---

## 3. Mode Resolution

```
EVIDRA_URL set?
├── NO → OFFLINE mode (local OPA, local evidence)
└── YES
    ├── --offline flag? → YES → OFFLINE mode
    └── NO
        ├── API reachable? → YES → ONLINE mode (POST to API)
        └── NO
            ├── EVIDRA_FALLBACK=offline? → YES → LOCAL-FALLBACK mode
            └── NO (default: closed) → ERROR (fail closed)
```

### Environment variables

| Variable | Default | Scope | Purpose |
|---|---|---|---|
| `EVIDRA_URL` | (unset) | CLI + MCP | API endpoint. Enables online mode. Example: `https://api.evidra.rest` |
| `EVIDRA_API_KEY` | (unset) | CLI + MCP | Bearer token. Required when `EVIDRA_URL` is set |
| `EVIDRA_FALLBACK` | `closed` | CLI + MCP | `closed` = error on API failure. `offline` = local eval on API failure |
| `EVIDRA_BUNDLE_PATH` | (embedded) | CLI + MCP | OPA bundle path for offline/fallback. If unset, uses embedded `ops-v0.1` |
| `EVIDRA_EVIDENCE_DIR` | `~/.evidra/evidence` | CLI + MCP | Local evidence path (offline/fallback only) |
| `EVIDRA_ENVIRONMENT` | (unset) | CLI + MCP | Environment name for `by_env` policy params |

### Flags

| Flag | Scope | Purpose |
|---|---|---|
| `--offline` | CLI + MCP | Force offline mode (skip API even if `EVIDRA_URL` set) |
| `--fallback-offline` | CLI + MCP | Same as `EVIDRA_FALLBACK=offline` |
| `--url <url>` | CLI | Override `EVIDRA_URL` for this invocation |
| `--api-key <key>` | CLI | Override `EVIDRA_API_KEY` for this invocation |

---

## 4. New Package: `pkg/client`

A thin HTTP client that POSTs to `EVIDRA_URL/v1/validate` and returns the same `validate.Result` struct.

### Interface

```go
package client

import (
    "context"
    "samebits.com/evidra/pkg/validate"
    "samebits.com/evidra/pkg/invocation"
)

// Config holds API connection settings.
type Config struct {
    URL     string        // e.g. "https://api.evidra.rest"
    APIKey  string        // Bearer token
    Timeout time.Duration // HTTP timeout (default: 30s)
}

// Client sends evaluation requests to the Evidra API.
type Client struct {
    config Config
    http   *http.Client
}

// New creates a new API client.
func New(cfg Config) *Client

// Validate sends a ToolInvocation to POST /v1/validate and returns a Result.
func (c *Client) Validate(ctx context.Context, inv invocation.ToolInvocation) (validate.Result, error)

// ValidateRaw sends a raw JSON body to POST /v1/validate.
// Used by CLI when evaluating scenario files (the file content is sent as-is).
func (c *Client) ValidateRaw(ctx context.Context, body []byte) (validate.Result, error)

// Ping checks if the API is reachable (GET /healthz).
// Returns nil if reachable, error if not.
func (c *Client) Ping(ctx context.Context) error
```

### Response mapping

The API returns JSON like:
```json
{
  "allow": true,
  "risk_level": "low",
  "reason": "scenario_validated",
  "reasons": [],
  "rule_ids": [],
  "hints": [],
  "evidence_id": "evt-...",
  "source": "api"
}
```

The client maps this to `validate.Result`:
```go
validate.Result{
    Pass:       response.Allow,
    RiskLevel:  response.RiskLevel,
    EvidenceID: response.EvidenceID,
    Reasons:    response.Reasons,
    RuleIDs:    response.RuleIDs,
    Hints:      response.Hints,
    Source:     "api",  // NEW FIELD
}
```

### Error handling

| HTTP Status | Behavior |
|---|---|
| 200 | Parse response, return Result (even if `allow: false`) |
| 401 | Return error: `ErrUnauthorized` (bad API key) |
| 403 | Return error: `ErrForbidden` |
| 422 | Return error: `ErrInvalidInput` (wraps server message) |
| 429 | Return error: `ErrRateLimited` |
| 5xx | Return error: `ErrServerError` (triggers fallback if configured) |
| Connection refused / timeout | Return error: `ErrUnreachable` (triggers fallback if configured) |

Only `ErrUnreachable` and `ErrServerError` trigger fallback. Auth errors (401/403) and validation errors (422) always fail immediately — no fallback.

---

## 5. New Package: `pkg/mode`

Encapsulates mode resolution logic. Used by both CLI and MCP.

```go
package mode

type Mode string

const (
    Online       Mode = "online"
    Offline      Mode = "offline"
    Fallback     Mode = "local-fallback"
)

// Config holds all mode-resolution inputs.
type Config struct {
    URL            string // from EVIDRA_URL or --url
    APIKey         string // from EVIDRA_API_KEY or --api-key
    FallbackPolicy string // from EVIDRA_FALLBACK: "closed" (default) or "offline"
    ForceOffline   bool   // from --offline flag
}

// Resolved holds the resolved mode and the client (if online).
type Resolved struct {
    Mode   Mode
    Client *client.Client // nil in offline mode
}

// Resolve determines the operating mode.
// If online and API is unreachable, applies fallback policy.
func Resolve(ctx context.Context, cfg Config) (Resolved, error)
```

Resolution logic:
1. If `cfg.ForceOffline` → return `Offline`, nil client
2. If `cfg.URL == ""` → return `Offline`, nil client
3. If `cfg.APIKey == ""` → return error: "EVIDRA_API_KEY required when EVIDRA_URL is set"
4. Create client, call `client.Ping(ctx)`
5. If ping OK → return `Online`, client
6. If ping fails:
   - If `cfg.FallbackPolicy == "offline"` → return `Fallback`, nil client
   - Else → return error: "API unreachable at <url>. Set EVIDRA_FALLBACK=offline to allow local evaluation"

---

## 6. Changes to `validate.Result`

Add a `Source` field to track where the evaluation happened:

```go
type Result struct {
    Pass       bool
    RiskLevel  string
    EvidenceID string
    Reasons    []string
    RuleIDs    []string
    Hints      []string
    Source     string // NEW: "api", "local", "local-fallback"
}
```

Local evaluation sets `Source: "local"`. API sets `Source: "api"`. Fallback sets `Source: "local-fallback"`.

The evidence record also gets the `source` field. In `pkg/evidence/types.go`, add `Source string` to `EvidenceRecord`. In `EvaluateScenario`, set it based on mode.

---

## 7. Changes to `cmd/evidra/main.go` (CLI)

### New flags on `validate` subcommand

```go
fs := flag.NewFlagSet("validate", flag.ContinueOnError)
// existing flags...
offlineFlag := fs.Bool("offline", false, "Force offline mode")
fallbackFlag := fs.Bool("fallback-offline", false, "Allow local evaluation when API unreachable")
urlFlag := fs.String("url", "", "Evidra API URL (overrides EVIDRA_URL)")
apiKeyFlag := fs.String("api-key", "", "API key (overrides EVIDRA_API_KEY)")
```

### Updated `runValidate` flow

```go
func runValidate(args []string, stdout, stderr io.Writer) int {
    // Parse flags...

    // 1. Resolve mode
    modeCfg := mode.Config{
        URL:            coalesce(*urlFlag, os.Getenv("EVIDRA_URL")),
        APIKey:         coalesce(*apiKeyFlag, os.Getenv("EVIDRA_API_KEY")),
        FallbackPolicy: coalesce(boolToFallback(*fallbackFlag), os.Getenv("EVIDRA_FALLBACK"), "closed"),
        ForceOffline:   *offlineFlag,
    }
    resolved, err := mode.Resolve(ctx, modeCfg)
    if err != nil {
        fmt.Fprintln(stderr, err.Error())
        return 1
    }

    // 2. Log mode to stderr (not stdout — stdout is for data)
    fmt.Fprintf(stderr, "mode: %s\n", resolved.Mode)

    var result validate.Result

    switch resolved.Mode {
    case mode.Online:
        // Read file, send to API
        data, err := os.ReadFile(path)
        if err != nil { ... }
        result, err = resolved.Client.ValidateRaw(ctx, data)
        if err != nil { ... }

    case mode.Offline, mode.Fallback:
        // Existing local evaluation path
        result, err = validate.EvaluateFile(ctx, path, validate.Options{
            PolicyPath:  *policyFlag,
            DataPath:    *dataFlag,
            BundlePath:  *bundleFlag,
            Environment: *envFlag,
        })
        if err != nil { ... }
        result.Source = string(resolved.Mode)
    }

    return printValidationResult(result, stdout, *jsonOut, *explain)
}
```

### Updated `printValidationResult`

Add `Source` to both human and JSON output:

```go
// Human output:
fmt.Fprintf(stdout, "Source: %s\n", result.Source)

// JSON output:
type validationJSON struct {
    // ...existing fields...
    Source string `json:"source"`
}
```

### Updated usage/help text

```
usage: evidra validate [flags] <file>

MODE FLAGS:
  --url <url>             Evidra API URL (overrides EVIDRA_URL)
  --api-key <key>         API key (overrides EVIDRA_API_KEY)
  --offline               Force offline mode (skip API)
  --fallback-offline      Allow local eval when API unreachable

POLICY FLAGS (offline/fallback only):
  --bundle <path>         OPA bundle directory
  --policy <path>         Policy rego file (loose mode)
  --data <path>           Policy data JSON (loose mode)
  --environment <env>     Environment label

OUTPUT FLAGS:
  --json                  Structured JSON output
  --explain               Human-readable explanation
```

---

## 8. Changes to `cmd/evidra-mcp/main.go`

### New flags

```go
offlineFlag := fs.Bool("offline", false, "Force offline mode")
fallbackFlag := fs.Bool("fallback-offline", false, "Allow local evaluation when API unreachable")
```

### Updated startup

```go
func run(args []string, stdout, stderr io.Writer) int {
    // Parse flags...

    // 1. Resolve mode
    modeCfg := mode.Config{
        URL:            os.Getenv("EVIDRA_URL"),
        APIKey:         os.Getenv("EVIDRA_API_KEY"),
        FallbackPolicy: coalesce(boolToFallback(*fallbackFlag), os.Getenv("EVIDRA_FALLBACK"), "closed"),
        ForceOffline:   *offlineFlag,
    }
    resolved, err := mode.Resolve(ctx, modeCfg)
    if err != nil {
        fmt.Fprintf(stderr, "%v\n", err)
        return 1
    }

    logger.Printf("evidra-mcp running in %s mode (%s)", mcpMode, resolved.Mode)

    // 2. Pass resolved mode to server options
    server := newServerFunc(mcpserver.Options{
        // ...existing fields...
        EvidraClient: resolved.Client, // nil in offline mode
        EvidraMode:   resolved.Mode,
    })
    // ...
}
```

### Changes to `pkg/mcpserver/server.go`

Add fields to `Options`:

```go
type Options struct {
    // ...existing fields...
    EvidraClient *client.Client // nil = offline mode
    EvidraMode   mode.Mode      // "online", "offline", "local-fallback"
}
```

Update `ValidateService`:

```go
type ValidateService struct {
    // ...existing fields...
    apiClient  *client.Client
    evidraMode mode.Mode
}

func (s *ValidateService) Validate(ctx context.Context, inv invocation.ToolInvocation) ValidateOutput {
    // ONLINE: delegate to API
    if s.apiClient != nil && s.evidraMode == mode.Online {
        result, err := s.apiClient.Validate(ctx, inv)
        if err != nil {
            // If unreachable and fallback configured, fall through to local
            if isReachabilityError(err) && s.evidraMode == mode.Fallback {
                // fall through to local evaluation below
            } else {
                return errorOutput(err)
            }
        } else {
            return resultToOutput(result)
        }
    }

    // OFFLINE or FALLBACK: local evaluation (existing code)
    res, err := validate.EvaluateInvocation(ctx, inv, validate.Options{
        PolicyPath:  s.policyPath,
        DataPath:    s.dataPath,
        BundlePath:  s.bundlePath,
        Environment: s.environment,
        EvidenceDir: s.evidencePath,
    })
    // ...existing response construction...
}
```

**Important:** In online mode, the MCP server does NOT need local bundle/policy config. The embedded bundle is only extracted in offline/fallback mode. This means the startup logic changes:

```go
// Only extract embedded bundle if NOT in online mode
if resolved.Mode != mode.Online {
    if bundlePath == "" && !looseMode {
        tmpDir, err := extractEmbeddedBundle(evidra.OpsV01BundleFS)
        // ...
    }
}
```

---

## 9. Changes to `pkg/config/config.go`

Add mode-related resolution functions:

```go
// ResolveURL returns the API URL from flag or env.
func ResolveURL(flagValue string) string {
    if v := strings.TrimSpace(flagValue); v != "" {
        return v
    }
    return strings.TrimSpace(os.Getenv("EVIDRA_URL"))
}

// ResolveAPIKey returns the API key from flag or env.
func ResolveAPIKey(flagValue string) string {
    if v := strings.TrimSpace(flagValue); v != "" {
        return v
    }
    return strings.TrimSpace(os.Getenv("EVIDRA_API_KEY"))
}

// ResolveFallback returns the fallback policy.
func ResolveFallback(flagValue bool) string {
    if flagValue {
        return "offline"
    }
    v := strings.TrimSpace(strings.ToLower(os.Getenv("EVIDRA_FALLBACK")))
    if v == "offline" {
        return "offline"
    }
    return "closed"
}
```

---

## 10. File Changes Summary

### New files

| File | Purpose |
|---|---|
| `pkg/client/client.go` | HTTP client: `Validate()`, `ValidateRaw()`, `Ping()` |
| `pkg/client/client_test.go` | Unit tests with `httptest.Server` |
| `pkg/client/errors.go` | Sentinel errors: `ErrUnreachable`, `ErrUnauthorized`, `ErrServerError`, etc. |
| `pkg/mode/mode.go` | Mode resolution: `Resolve()` returns `Resolved{Mode, Client}` |
| `pkg/mode/mode_test.go` | Unit tests for resolution logic |

### Modified files

| File | Changes |
|---|---|
| `pkg/validate/validate.go` | Add `Source` field to `Result` struct. Set `Source: "local"` in `EvaluateScenario` |
| `pkg/evidence/types.go` | Add `Source string` to `EvidenceRecord` |
| `pkg/config/config.go` | Add `ResolveURL()`, `ResolveAPIKey()`, `ResolveFallback()` |
| `cmd/evidra/main.go` | Add `--offline`, `--fallback-offline`, `--url`, `--api-key` flags. Mode resolution before evaluation. Updated help text |
| `cmd/evidra-mcp/main.go` | Add `--offline`, `--fallback-offline` flags. Mode resolution at startup. Skip embedded bundle extraction in online mode |
| `pkg/mcpserver/server.go` | Add `EvidraClient` and `EvidraMode` to Options/ValidateService. Online path delegates to API client |

### NOT modified

| File | Reason |
|---|---|
| `pkg/runtime/runtime.go` | Pure OPA evaluation, unchanged |
| `pkg/policy/policy.go` | OPA wrapper, unchanged |
| `pkg/scenario/load.go` | Scenario loading, unchanged |
| `pkg/bundlesource/` | Bundle source, unchanged |
| `pkg/evidence/evidence.go` | Local evidence store, unchanged (still used in offline/fallback) |
| `cmd/evidra/evidence_cmd.go` | Evidence inspect/report commands stay local-only |
| `cmd/evidra/policy_sim_cmd.go` | Policy sim stays local-only (it's for testing policy, not for production eval) |

---

## 11. Testing Strategy

### Unit tests

**`pkg/client/client_test.go`:**
- `TestValidate_Success`: mock server returns 200 + allow=true → Result with Source="api"
- `TestValidate_Denied`: mock server returns 200 + allow=false → Result with Pass=false
- `TestValidate_Unauthorized`: mock server returns 401 → ErrUnauthorized
- `TestValidate_ServerError`: mock server returns 500 → ErrServerError
- `TestValidate_Unreachable`: no server running → ErrUnreachable
- `TestValidate_Timeout`: slow server → ErrUnreachable (context deadline)
- `TestPing_Success`: mock /healthz returns 200
- `TestPing_Unreachable`: no server → error

**`pkg/mode/mode_test.go`:**
- `TestResolve_NoURL`: URL empty → Offline mode
- `TestResolve_ForceOffline`: URL set + --offline → Offline mode
- `TestResolve_NoAPIKey`: URL set, no key → error
- `TestResolve_Online`: URL + key + API up → Online mode
- `TestResolve_FallbackClosed`: URL + key + API down + fallback=closed → error
- `TestResolve_FallbackOffline`: URL + key + API down + fallback=offline → Fallback mode

### Integration tests

**`cmd/evidra/main_test.go` (add):**
- `TestValidate_OnlineMode`: start test HTTP server, set EVIDRA_URL, run CLI → result from server
- `TestValidate_OfflineFlag`: set EVIDRA_URL + --offline → local evaluation
- `TestValidate_FallbackClosed`: set EVIDRA_URL to dead server → exit 1
- `TestValidate_FallbackOffline`: set EVIDRA_URL to dead server + EVIDRA_FALLBACK=offline → local eval

**`cmd/evidra-mcp/test/` (add):**
- `TestMCP_OnlineMode`: start test HTTP server, set EVIDRA_URL, send validate fixture → response from server
- `TestMCP_OfflineMode`: unset EVIDRA_URL → response from local OPA

### Existing tests

All existing tests continue to work unchanged — they don't set `EVIDRA_URL`, so they run in offline mode (the default). No test regressions.

---

## 12. Implementation Steps (for Claude Code)

### Prerequisites

Read and understand:
1. This document (the one you're reading)
2. `CLAUDE.md` in the repo root
3. The existing source files listed in §10

### Step 1: Add `Source` field to `validate.Result`

**Files:** `pkg/validate/validate.go`

Add `Source string` to `Result` struct. Set `Source: "local"` in `EvaluateScenario` after evaluation (before return). This is backward-compatible — existing code ignores the field.

**Also:** Add `Source string` field to `pkg/evidence/types.go` `EvidenceRecord`. Set it from the Result's Source in `EvaluateScenario` when building the record.

**Acceptance:** `go test ./pkg/validate/... ./pkg/evidence/...` passes. Existing tests unaffected.

---

### Step 2: Create `pkg/client`

**Files:** `pkg/client/client.go`, `pkg/client/errors.go`, `pkg/client/client_test.go`

Implement:
- `Config` struct
- `New(cfg Config) *Client`
- `Validate(ctx, inv) (validate.Result, error)` — POST to `/v1/validate`
- `ValidateRaw(ctx, body) (validate.Result, error)` — POST raw JSON to `/v1/validate`
- `Ping(ctx) error` — GET `/healthz`
- Sentinel errors: `ErrUnreachable`, `ErrUnauthorized`, `ErrForbidden`, `ErrRateLimited`, `ErrServerError`, `ErrInvalidInput`

Request format for `Validate`:
```json
{
  "actor": {"type": "...", "id": "...", "origin": "..."},
  "tool": "...",
  "operation": "...",
  "params": {...},
  "context": {...}
}
```

Response parsing: API returns `allow`, `risk_level`, `reason`, `reasons`, `rule_ids`, `hints`, `evidence_id`. Map to `validate.Result` with `Source: "api"`.

Default timeout: 30 seconds. Use `context.Context` for cancellation.

**Dependencies:** Only `net/http`, `encoding/json`, standard library. No external HTTP client library.

**Acceptance:** `go test -race ./pkg/client/...` — all pass. Tests use `httptest.NewServer`.

---

### Step 3: Create `pkg/mode`

**Files:** `pkg/mode/mode.go`, `pkg/mode/mode_test.go`

Implement:
- `Mode` type (string constant)
- `Config` struct
- `Resolved` struct
- `Resolve(ctx, cfg) (Resolved, error)`

Resolution logic from §5. Uses `client.New()` and `client.Ping()` to check reachability.

**Acceptance:** `go test -race ./pkg/mode/...` — all pass. Tests use `httptest.NewServer` for reachability checks.

---

### Step 4: Update `pkg/config` 

**File:** `pkg/config/config.go`

Add `ResolveURL()`, `ResolveAPIKey()`, `ResolveFallback()` as described in §9.

**Acceptance:** `go test -race ./pkg/config/...` passes.

---

### Step 5: Update CLI (`cmd/evidra/main.go`)

Add flags: `--offline`, `--fallback-offline`, `--url`, `--api-key`.

Update `runValidate`:
1. Build `mode.Config` from flags and env vars
2. Call `mode.Resolve(ctx, cfg)` 
3. Switch on mode: Online → use client; Offline/Fallback → existing local eval
4. Print mode to stderr
5. Include `Source` in output

Update help text.

**Do NOT change** `evidence` or `policy sim` subcommands — they stay local-only.

**Acceptance:** 
- `go build -o bin/evidra ./cmd/evidra` succeeds
- Existing `cmd/evidra` tests pass (they run in offline mode)
- New integration tests pass (see §11)

---

### Step 6: Update MCP server

**Files:** `cmd/evidra-mcp/main.go`, `pkg/mcpserver/server.go`

In `main.go`:
1. Add `--offline`, `--fallback-offline` flags
2. Resolve mode at startup
3. Only extract embedded bundle in offline/fallback mode
4. Pass `client.Client` and `mode.Mode` to `mcpserver.Options`

In `server.go`:
1. Add `EvidraClient` and `EvidraMode` to `Options` and `ValidateService`
2. In `Validate()`: if online mode and client is set, delegate to `client.Validate()`
3. If API call fails with reachability error and fallback mode, fall through to local eval
4. If offline mode, use existing local evaluation path

**Acceptance:**
- `go build -o bin/evidra-mcp ./cmd/evidra-mcp` succeeds
- Existing MCP tests pass (no EVIDRA_URL set → offline mode)
- New MCP integration tests pass

---

### Step 7: Update CLAUDE.md

Add to Architecture section:

```markdown
### Hybrid mode (API-first)

When `EVIDRA_URL` is set, CLI and MCP delegate evaluation to the API server (`pkg/client`).
When unset or `--offline`, they evaluate locally using the embedded OPA bundle.

Mode resolution: `pkg/mode.Resolve()` → `Online` | `Offline` | `Fallback`

- **Online**: POST to `EVIDRA_URL/v1/validate`. Server policy is source of truth.
- **Offline**: Local OPA + local evidence. Fully autonomous.
- **Fallback**: API configured but unreachable + `EVIDRA_FALLBACK=offline`. Local eval, evidence marked `source: local-fallback`.

Default fallback: **fail closed** (error when API unreachable).
```

Update env vars section to include `EVIDRA_URL`, `EVIDRA_API_KEY`, `EVIDRA_FALLBACK`.

---

## 13. Checklist Before Merge

```
[ ] go test -race -cover ./... — all pass
[ ] go vet ./... — clean
[ ] gofmt -d . — no diff
[ ] go mod tidy — no diff
[ ] Existing tests unaffected (offline mode is default)
[ ] New tests cover: online, offline, fallback-closed, fallback-offline
[ ] pkg/client has no external dependencies (stdlib only)
[ ] CLI --help shows new flags
[ ] MCP --help shows new flags
[ ] CLAUDE.md updated
[ ] Source field present in validate.Result and evidence records
[ ] evidence and policy sim subcommands unchanged
```

---

## 14. What This Does NOT Include

- **`evidra-api` changes**: The API server is unchanged. It already has `/v1/validate` and `/healthz`.
- **Evidence sync** (`evidra evidence sync`): Phase 2. Requires a new API endpoint for evidence ingestion.
- **Skills support in CLI**: Phase 2. CLI currently only does raw validation, not skill-based execution.
- **MCP tools beyond validate**: `get_event` stays local-only for now. Online `get_event` would need API endpoint.
- **Auth token rotation**: Out of scope. CLI/MCP use a static `EVIDRA_API_KEY`.
