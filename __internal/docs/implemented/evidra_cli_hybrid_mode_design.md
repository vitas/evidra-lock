# Evidra — Hybrid Mode Refactoring Design

**Date:** 2026-02-26
**Status:** Ready for implementation (v2 — post-review)
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
| 4 | MCP in online mode talks to Evidra API via `EVIDRA_URL`. Often deployed as localhost sidecar, but works with any reachable API | MCP stays focused on stdio transport. API handles auth, policy, evidence, skills. No localhost requirement |
| 5 | Evidence from fallback marked `"source": "local-fallback"` | Auditors can distinguish server-authoritative vs degraded-mode decisions |
| 6 | `evidra evidence sync` is Phase 2, not in this refactor | Sync requires API evidence ingestion endpoint that doesn't exist yet |
| 7 | `--offline` flag forces offline mode even when `EVIDRA_URL` is set | Developer convenience: quick local testing without unsetting env vars |
| 8 | New `pkg/client` for HTTP client | Clean separation. No HTTP in `pkg/validate` (stays pure) |
| 9 | **No reachability ping on hot path** | Ping before every validate doubles latency and adds a failure point. Try the API call directly; handle errors at response time |
| 10 | **Fallback is a policy, not a mode** | `Resolve()` returns `Online` or `Offline`. Fallback happens at runtime when an Online call fails — not as a pre-resolved state |
| 11 | **CLI always sends ToolInvocation, never raw scenario** | The API only understands ToolInvocation. CLI loads the scenario file locally, converts to ToolInvocation(s), and POSTs each one. No ambiguous "raw" passthrough |
| 12 | **Separate response struct in client** | `apiValidateResponse` (private) decouples from server schema evolution. Unknown fields silently ignored. Maps to `validate.Result` |
| 13 | **`X-Request-ID` on every API call** | Random UUID per request, logged to stderr in `--verbose`. Correlates client ↔ server logs |
| 14 | **`actionToInvocation` keys must match `ValidateStructure` allowlist** | If a key isn't in `allowedParamKeys`, the server will silently ignore it. Pre-flight assertion in tests |
| 15 | **Embedded bundle cached to `~/.evidra/bundles/`** | Deterministic path avoids tmpdir spam on MCP restarts. No-op if cache is fresh |

---

## 3. Mode Resolution

Resolve determines the **intent** (Online or Offline). Fallback is a **runtime behavior**, not a resolved mode.

```
EVIDRA_URL set?
├── NO → Offline (local OPA, local evidence)
└── YES
    ├── --offline flag? → YES → Offline
    └── NO
        ├── EVIDRA_API_KEY set? → NO → error: "EVIDRA_API_KEY required"
        └── YES → Online (with fallbackPolicy from EVIDRA_FALLBACK)
```

At runtime (inside each `Validate` call):
```
POST /v1/validate →
├── 200 → return Result{Source: "api"}
├── 401/403/422 → return error immediately (no fallback)
├── 5xx / connect error / timeout →
│   ├── fallbackPolicy == "offline" → evaluate locally, Source: "local-fallback"
│   └── fallbackPolicy == "closed" (default) → return error (exit code 3)
└── 429 → return error immediately (no fallback)
```

**Key insight:** No separate "Fallback mode" state. There's Online and Offline. When you're Online and the API fails, the `fallbackPolicy` setting determines what happens next — it's a degradation behavior, not a mode.

### Environment variables

| Variable | Default | Scope | Purpose |
|---|---|---|---|
| `EVIDRA_URL` | (unset) | CLI + MCP | API endpoint. Enables online mode. Example: `https://evidra.rest` |
| `EVIDRA_API_KEY` | (unset) | CLI + MCP | Bearer token. Required when `EVIDRA_URL` is set |
| `EVIDRA_FALLBACK` | `closed` | CLI + MCP | `closed` = error on API failure. `offline` = local eval on API failure |
| `EVIDRA_BUNDLE_PATH` | (embedded) | CLI + MCP | OPA bundle path for offline/fallback. If unset, uses embedded `ops-v0.1` |
| `EVIDRA_EVIDENCE_DIR` | `~/.evidra/evidence` | CLI + MCP | Local evidence path (offline/fallback only) |
| `EVIDRA_ENVIRONMENT` | (unset) | CLI + MCP | Environment name for `by_env` policy params. Normalized: `prod`/`prd` → `production`, `stg`/`stage` → `staging` |

### Flags

| Flag | Scope | Purpose |
|---|---|---|
| `--offline` | CLI + MCP | Force offline mode (skip API even if `EVIDRA_URL` set) |
| `--fallback-offline` | CLI + MCP | Same as `EVIDRA_FALLBACK=offline` |
| `--url <url>` | CLI | Override `EVIDRA_URL` for this invocation |
| `--api-key <key>` | CLI | Override `EVIDRA_API_KEY` for this invocation |
| `--timeout <duration>` | CLI | HTTP timeout for API calls (default: 30s, e.g. `--timeout 10s`) |

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (policy allowed) |
| 1 | Internal error (policy failure, evidence write, auth error, etc.) |
| 2 | **Policy denied** |
| 3 | **API unreachable** (online mode, fallback=closed) |
| 4 | **Usage error** (bad flags, missing file, invalid input) |

---

## 4. New Package: `pkg/client`

A thin HTTP client that POSTs to `EVIDRA_URL/v1/validate`. No reachability probing — just fire the request and handle the response.

### Interface

```go
package client

import (
    "context"
    "net/http"
    "time"

    "samebits.com/evidra/pkg/invocation"
    "samebits.com/evidra/pkg/validate"
)

// Config holds API connection settings.
type Config struct {
    URL     string        // e.g. "https://evidra.rest"
    APIKey  string        // Bearer token
    Timeout time.Duration // HTTP timeout (default: 30s)
}

// Client sends evaluation requests to the Evidra API.
type Client struct {
    config Config
    http   *http.Client
}

// New creates a new API client. Does NOT check reachability.
func New(cfg Config) *Client

// URL returns the configured API URL (for error messages).
func (c *Client) URL() string

// Validate sends a ToolInvocation to POST /v1/validate and returns a Result.
// Sets X-Request-ID header (random UUID) for server log correlation.
// On connect/timeout/5xx errors, returns ErrUnreachable or ErrServerError
// (caller decides whether to fall back to local eval).
// Returns (Result, requestID, error) — requestID is logged by caller in verbose mode.
func (c *Client) Validate(ctx context.Context, inv invocation.ToolInvocation) (validate.Result, string, error)

// Ping checks if the API is reachable (GET /healthz).
// NOT used on the hot path. Reserved for `evidra doctor` or
// optional one-time startup check.
func (c *Client) Ping(ctx context.Context) error
```

### Internal response struct

The client decodes the API response into a private struct, not `validate.Result` directly. This decouples the client from server schema evolution.

```go
// apiValidateResponse is the JSON shape returned by POST /v1/validate.
// Private — only used for deserialization. Unknown fields are silently ignored.
type apiValidateResponse struct {
    Allow      bool     `json:"allow"`
    RiskLevel  string   `json:"risk_level"`
    Reason     string   `json:"reason"`
    Reasons    []string `json:"reasons"`
    RuleIDs    []string `json:"rule_ids"`
    Hints      []string `json:"hints"`
    EvidenceID string   `json:"evidence_id"`
    PolicyRef  string   `json:"policy_ref"`
    // Future fields from server are silently ignored
}

// toResult maps apiValidateResponse → validate.Result.
func (r *apiValidateResponse) toResult() validate.Result {
    return validate.Result{
        Pass:       r.Allow,
        RiskLevel:  r.RiskLevel,
        EvidenceID: r.EvidenceID,
        Reasons:    r.Reasons,
        RuleIDs:    r.RuleIDs,
        Hints:      r.Hints,
        Source:     "api",
        PolicyRef:  r.PolicyRef,
    }
}
```

### Error handling

```go
// Sentinel errors — use errors.Is() to check.
var (
    ErrUnreachable  = errors.New("api_unreachable")   // connect refused, DNS, timeout
    ErrUnauthorized = errors.New("unauthorized")       // 401
    ErrForbidden    = errors.New("forbidden")          // 403
    ErrRateLimited  = errors.New("rate_limited")       // 429
    ErrServerError  = errors.New("server_error")       // 5xx
    ErrInvalidInput = errors.New("invalid_input")      // 422
)

// IsReachabilityError returns true for errors that can trigger fallback.
// Only ErrUnreachable and ErrServerError qualify.
// Auth errors (401/403), validation (422), and rate limit (429) always fail immediately.
func IsReachabilityError(err error) bool {
    return errors.Is(err, ErrUnreachable) || errors.Is(err, ErrServerError)
}
```

**Error classification in `Validate()`:** The client must precisely map HTTP failures:

```go
// classifyError maps transport/HTTP errors to sentinel errors.
func classifyError(err error, resp *http.Response) error {
    // Transport-level failures → ErrUnreachable
    if err != nil {
        // context.DeadlineExceeded, context.Canceled, net.Error timeout,
        // connection refused, DNS resolution failure
        var netErr net.Error
        if errors.Is(err, context.DeadlineExceeded) ||
           errors.Is(err, context.Canceled) ||
           (errors.As(err, &netErr) && netErr.Timeout()) {
            return fmt.Errorf("%w: %v", ErrUnreachable, err)
        }
        // All other transport errors (connection refused, DNS, etc.)
        return fmt.Errorf("%w: %v", ErrUnreachable, err)
    }

    // HTTP status codes
    switch {
    case resp.StatusCode == 401:
        return ErrUnauthorized
    case resp.StatusCode == 403:
        return ErrForbidden
    case resp.StatusCode == 422:
        return ErrInvalidInput
    case resp.StatusCode == 429:
        return ErrRateLimited
    case resp.StatusCode >= 500:
        return fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
    default:
        return nil
    }
}
```

**Test coverage for error taxonomy:**
- `TestValidate_Timeout`: context with short deadline → `ErrUnreachable`, `IsReachabilityError()==true`
- `TestValidate_ContextCanceled`: canceled context → `ErrUnreachable`
- `TestValidate_429_NoFallback`: mock 429 → `ErrRateLimited`, `IsReachabilityError()==false`

**Dependencies:** Only standard library (`net/http`, `encoding/json`, `errors`, `time`, `context`). No external HTTP client library.

---

## 5. New Package: `pkg/mode`

Encapsulates startup-time mode resolution. Does NOT check reachability — that happens at call time.

```go
package mode

import (
    "fmt"
    "strings"

    "samebits.com/evidra/pkg/client"
)

// Resolved holds the resolved mode and runtime config.
type Resolved struct {
    IsOnline       bool           // true = EVIDRA_URL is set and not --offline
    Client         *client.Client // non-nil only when IsOnline=true
    FallbackPolicy string         // "closed" (default) or "offline"
}

// Config holds all mode-resolution inputs.
type Config struct {
    URL            string // from EVIDRA_URL or --url
    APIKey         string // from EVIDRA_API_KEY or --api-key
    FallbackPolicy string // from EVIDRA_FALLBACK: "closed" (default) or "offline"
    ForceOffline   bool   // from --offline flag
}

// Resolve determines the operating mode. Does NOT ping the API.
// Returns error only for invalid configuration (e.g. URL set but no API key).
func Resolve(cfg Config) (Resolved, error) {
    fallback := normalizeFallback(cfg.FallbackPolicy)

    // 1. Force offline or no URL
    if cfg.ForceOffline || strings.TrimSpace(cfg.URL) == "" {
        return Resolved{IsOnline: false, FallbackPolicy: fallback}, nil
    }

    // 2. Online requires API key
    if strings.TrimSpace(cfg.APIKey) == "" {
        return Resolved{}, fmt.Errorf("EVIDRA_API_KEY is required when EVIDRA_URL is set")
    }

    // 3. Create client (no ping, no I/O)
    c := client.New(client.Config{
        URL:    strings.TrimSpace(cfg.URL),
        APIKey: strings.TrimSpace(cfg.APIKey),
    })

    return Resolved{
        IsOnline:       true,
        Client:         c,
        FallbackPolicy: fallback,
    }, nil
}

func normalizeFallback(v string) string {
    if strings.ToLower(strings.TrimSpace(v)) == "offline" {
        return "offline"
    }
    return "closed"
}
```

**Note:** `Resolve` is synchronous, instant, and never does I/O. It's just config validation + client construction. All network happens later in `client.Validate()`.

---

## 6. Changes to `validate.Result`

Add `Source` and `PolicyRef` fields:

```go
type Result struct {
    Pass        bool
    RiskLevel   string
    EvidenceID  string   // Single-action shortcut (most common case)
    EvidenceIDs []string // NEW: all evidence IDs for multi-action scenarios
    RequestIDs  []string // NEW: X-Request-ID per API call (online only, empty in offline)
    Reasons     []string
    RuleIDs     []string
    Hints       []string
    Source      string   // NEW: "api", "local", "local-fallback"
    PolicyRef   string   // NEW: policy bundle ref (from server or local)
}
```

In `EvaluateScenario`, set `Source: "local"` and `PolicyRef` from the evaluator before return. The API client sets `Source: "api"`. Fallback code sets `Source: "local-fallback"`.

Also add `Source string` to `pkg/evidence/types.go` `EvidenceRecord`.

---

## 7. Changes to `cmd/evidra/main.go` (CLI)

### New flags on `validate` subcommand

```go
offlineFlag  := fs.Bool("offline", false, "Force offline mode")
fallbackFlag := fs.Bool("fallback-offline", false, "Allow local eval when API unreachable")
urlFlag      := fs.String("url", "", "Evidra API URL (overrides EVIDRA_URL)")
apiKeyFlag   := fs.String("api-key", "", "API key (overrides EVIDRA_API_KEY)")
```

### Updated `runValidate` flow

```go
func runValidate(args []string, stdout, stderr io.Writer) int {
    // Parse flags...

    // 1. Resolve mode (instant, no I/O)
    resolved, err := mode.Resolve(mode.Config{
        URL:            coalesce(*urlFlag, os.Getenv("EVIDRA_URL")),
        APIKey:         coalesce(*apiKeyFlag, os.Getenv("EVIDRA_API_KEY")),
        FallbackPolicy: fallbackValue(*fallbackFlag),
        ForceOffline:   *offlineFlag,
    })
    if err != nil {
        fmt.Fprintln(stderr, err.Error())
        return 1
    }

    modeLabel := "offline"
    if resolved.IsOnline {
        modeLabel = "online"
    }
    fmt.Fprintf(stderr, "mode: %s\n", modeLabel)

    // 2. Load scenario file locally (always — we need ToolInvocations for online path)
    sc, err := scenario.LoadFile(path)
    if err != nil {
        fmt.Fprintln(stderr, err.Error())
        return 1
    }

    var result validate.Result

    if resolved.IsOnline {
        // 3a. Online: scenario → ToolInvocation(s) → POST each to API
        result, err = evaluateOnline(ctx, resolved, sc, opts)
        if err != nil {
            if client.IsReachabilityError(err) && resolved.FallbackPolicy == "offline" {
                fmt.Fprintf(stderr, "API unreachable, falling back to local evaluation\n")
                result, err = evaluateLocal(ctx, sc, opts)
                if err != nil {
                    fmt.Fprintln(stderr, err.Error())
                    return 1
                }
                result.Source = "local-fallback"
            } else if client.IsReachabilityError(err) {
                // Fail closed (default)
                if *jsonOut {
                    printJSONError(stdout, "API_UNREACHABLE",
                        fmt.Sprintf("API unreachable at %s", resolved.Client.URL()))
                }
                fmt.Fprintf(stderr, "error: API unreachable at %s\n", resolved.Client.URL())
                fmt.Fprintf(stderr, "hint: set EVIDRA_FALLBACK=offline to allow local evaluation\n")
                return 3
            } else {
                fmt.Fprintln(stderr, err.Error())
                return 1
            }
        }
    } else {
        // 3b. Offline: existing local evaluation
        result, err = evaluateLocal(ctx, sc, opts)
        if err != nil {
            fmt.Fprintln(stderr, err.Error())
            return 1
        }
    }

    return printValidationResult(result, stdout, *jsonOut, *explain)
}

// evaluateOnline converts scenario actions to ToolInvocations and sends each to API.
// Each action gets its own server-side evaluation and evidence record.
func evaluateOnline(ctx context.Context, resolved mode.Resolved, sc scenario.Scenario, opts localOpts) (validate.Result, error) {
    aggregate := validate.Result{Pass: true, RiskLevel: "low", Source: "api"}

    for _, action := range sc.Actions {
        inv := actionToInvocation(sc, action, opts.Environment)
        result, reqID, err := resolved.Client.Validate(ctx, inv)
        if opts.Verbose {
            fmt.Fprintf(opts.Stderr, "  request_id=%s action=%s\n", reqID, action.Kind)
        }
        if err != nil {
            return validate.Result{}, err // caller handles fallback
        }
        if !result.Pass {
            aggregate.Pass = false
        }
        aggregate.RiskLevel = maxRiskLevel(aggregate.RiskLevel, result.RiskLevel)
        aggregate.Reasons = append(aggregate.Reasons, result.Reasons...)
        aggregate.RuleIDs = append(aggregate.RuleIDs, result.RuleIDs...)
        aggregate.Hints = append(aggregate.Hints, result.Hints...)
        aggregate.EvidenceIDs = append(aggregate.EvidenceIDs, result.EvidenceID)
        aggregate.RequestIDs = append(aggregate.RequestIDs, reqID)
        if aggregate.PolicyRef == "" {
            aggregate.PolicyRef = result.PolicyRef
        } else if result.PolicyRef != "" && result.PolicyRef != aggregate.PolicyRef {
            // Different policy refs across actions = control-plane rollout in progress.
            // Log warning; set to "mixed" so callers know results span policy versions.
            if opts.Verbose {
                fmt.Fprintf(opts.Stderr, "  warning: policy_ref changed mid-scenario: %s → %s\n",
                    aggregate.PolicyRef, result.PolicyRef)
            }
            aggregate.PolicyRef = "mixed"
        }
    }

    // Single-action shortcut: promote to EvidenceID for backward compat
    if len(aggregate.EvidenceIDs) == 1 {
        aggregate.EvidenceID = aggregate.EvidenceIDs[0]
    }

    // Dedupe RuleIDs and Hints (duplicates spam multi-action output).
    // Reasons kept as-is for per-action context.
    aggregate.RuleIDs = dedupeStrings(aggregate.RuleIDs)
    aggregate.Hints = dedupeStrings(aggregate.Hints)

    return aggregate, nil
}

// dedupeStrings removes duplicates preserving order.
func dedupeStrings(ss []string) []string {
    if len(ss) <= 1 {
        return ss
    }
    seen := make(map[string]struct{}, len(ss))
    out := make([]string, 0, len(ss))
    for _, s := range ss {
        if _, ok := seen[s]; !ok {
            seen[s] = struct{}{}
            out = append(out, s)
        }
    }
    return out
}

// riskLevelPriority defines ordering: low < medium < high < critical.
var riskLevelPriority = map[string]int{
    "low": 0, "medium": 1, "high": 2, "critical": 3,
}

// maxRiskLevel returns the higher of two risk levels.
func maxRiskLevel(a, b string) string {
    pa, oa := riskLevelPriority[a]
    pb, ob := riskLevelPriority[b]
    if !oa { pa = -1 }
    if !ob { pb = -1 }
    if pb > pa {
        return b
    }
    return a
}

// evaluateLocal uses existing pkg/validate path.
func evaluateLocal(ctx context.Context, sc scenario.Scenario, opts localOpts) (validate.Result, error) {
    return validate.EvaluateScenario(ctx, sc, validate.Options{
        PolicyPath:  opts.PolicyPath,
        DataPath:    opts.DataPath,
        BundlePath:  opts.BundlePath,
        Environment: opts.Environment,
    })
}

// actionToInvocation converts a scenario.Action to invocation.ToolInvocation.
// Same conversion that EvaluateScenario does internally, exposed for online path.
func actionToInvocation(sc scenario.Scenario, action scenario.Action, env string) invocation.ToolInvocation {
    tool, operation, _ := splitKind(action.Kind)
    return invocation.ToolInvocation{
        Actor: invocation.Actor{
            Type:   sc.Actor.Type,
            ID:     coalesce(sc.Actor.ID, sc.ScenarioID),
            Origin: sc.Source,
        },
        Tool:        tool,
        Operation:   operation,
        Environment: env,
        Params: map[string]interface{}{
            invocation.KeyTarget:   action.Target,
            invocation.KeyPayload:  action.Payload,
            invocation.KeyRiskTags: action.RiskTags,
            invocation.KeyIntent:   action.Intent,
        },
        Context: map[string]interface{}{
            invocation.KeyScenarioID: sc.ScenarioID,
            invocation.KeySource:     sc.Source,
        },
    }
}
```

### Updated JSON output

```go
type validationJSON struct {
    Status      string     `json:"status"`
    Mode        string     `json:"mode"`
    RiskLevel   string     `json:"risk_level"`
    Reason      string     `json:"reason"`
    Reasons     []string   `json:"reasons,omitempty"`
    RuleIDs     []string   `json:"rule_ids,omitempty"`
    Hints       []string   `json:"hints,omitempty"`
    EvidenceID  string     `json:"evidence_id,omitempty"`
    EvidenceIDs []string   `json:"evidence_ids,omitempty"`
    Source      string     `json:"source"`
    PolicyRef   string     `json:"policy_ref,omitempty"`
    RequestID   string     `json:"request_id,omitempty"`
    Timestamp   string     `json:"timestamp"`
    Error       *errorJSON `json:"error,omitempty"`
}

type errorJSON struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    URL     string `json:"url,omitempty"`
}
```

Exit code 3 with `--json`:
```json
{
  "status": "ERROR",
  "mode": "online",
  "error": {
    "code": "API_UNREACHABLE",
    "message": "API unreachable at https://evidra.rest",
    "url": "https://evidra.rest"
  },
  "source": "none"
}
```

### Updated help text

```
usage: evidra validate [flags] <file>

CONNECTION FLAGS:
  --url <url>             Evidra API URL (overrides EVIDRA_URL)
  --api-key <key>         API key (overrides EVIDRA_API_KEY)
  --timeout <duration>    HTTP timeout (default: 30s)
  --offline               Force offline mode (skip API)
  --fallback-offline      Allow local eval when API unreachable

POLICY FLAGS (offline/fallback only):
  --bundle <path>         OPA bundle directory
  --policy <path>         Policy rego file (loose mode)
  --data <path>           Policy data JSON (loose mode)
  --environment <env>     Environment label (normalized: prod→production, stg→staging)

OUTPUT FLAGS:
  --json                  Structured JSON output
  --explain               Human-readable explanation
  --verbose               Log request IDs, mode, timing to stderr

EXIT CODES:
  0  Policy allowed
  1  Internal error (policy failure, auth error)
  2  Policy denied
  3  API unreachable (online mode, fallback=closed)
  4  Usage error (bad flags, missing file, invalid input)
```

---

## 8. Changes to `cmd/evidra-mcp/main.go`

### New flags

```go
offlineFlag  := fs.Bool("offline", false, "Force offline mode")
fallbackFlag := fs.Bool("fallback-offline", false, "Allow local eval when API unreachable")
```

### Updated startup

```go
func run(args []string, stdout, stderr io.Writer) int {
    // Parse flags...

    // 1. Resolve mode (instant, no I/O)
    resolved, err := mode.Resolve(mode.Config{
        URL:            os.Getenv("EVIDRA_URL"),
        APIKey:         os.Getenv("EVIDRA_API_KEY"),
        FallbackPolicy: fallbackValue(*fallbackFlag),
        ForceOffline:   *offlineFlag,
    })
    if err != nil {
        fmt.Fprintf(stderr, "%v\n", err)
        return 1
    }

    // 2. Extract embedded bundle when needed for local evaluation
    //    Online + fallback=offline: need bundle ready for potential runtime fallback
    //    Offline: always need bundle
    needLocalBundle := !resolved.IsOnline || resolved.FallbackPolicy == "offline"

    if needLocalBundle {
        bundlePath := config.ResolveBundlePath(*bundleFlag)
        looseMode := /* ...existing check... */
        if bundlePath == "" && !looseMode {
            // Cache to deterministic path to avoid tmpdir spam across restarts.
            // extractEmbeddedBundle writes to ~/.evidra/bundles/ops-v0.1/
            // and is a no-op if the dir already exists with matching content.
            cachedPath, err := extractEmbeddedBundle(evidra.OpsV01BundleFS, bundleCachePath())
            if err != nil { /* ... */ }
            fmt.Fprintln(stderr, "using built-in ops-v0.1 bundle")
            bundlePath = cachedPath
        }
    }

    // 3. Pass to server options
    server := newServerFunc(mcpserver.Options{
        // ...existing fields (bundlePath, policyPath, etc.)...
        APIClient:      resolved.Client,
        FallbackPolicy: resolved.FallbackPolicy,
        IsOnline:       resolved.IsOnline,
    })
    // ...
}
```

---

## 9. Changes to `pkg/mcpserver/server.go`

Add fields to `Options` and `ValidateService`:

```go
type Options struct {
    // ...existing fields...
    APIClient      *client.Client
    FallbackPolicy string
    IsOnline       bool
}

type ValidateService struct {
    // ...existing fields...
    apiClient      *client.Client
    fallbackPolicy string
    isOnline       bool
}
```

Updated `Validate`:

```go
func (s *ValidateService) Validate(ctx context.Context, inv invocation.ToolInvocation) ValidateOutput {
    // ONLINE: try API first (single call, no ping)
    if s.isOnline && s.apiClient != nil {
        result, _, err := s.apiClient.Validate(ctx, inv)
        if err == nil {
            return resultToValidateOutput(result)
        }

        // Reachability error → check fallback policy
        if client.IsReachabilityError(err) && s.fallbackPolicy == "offline" {
            // Fall through to local evaluation below
        } else {
            // Non-recoverable: auth/validation/rate-limit, or fallback=closed
            return errorOutput(err)
        }
    }

    // OFFLINE or FALLBACK: local evaluation (existing code path)
    res, err := validate.EvaluateInvocation(ctx, inv, validate.Options{
        PolicyPath:  s.policyPath,
        DataPath:    s.dataPath,
        BundlePath:  s.bundlePath,
        Environment: s.environment,
        EvidenceDir: s.evidencePath,
    })
    if err != nil {
        // ...existing error handling...
    }

    source := "local"
    if s.isOnline {
        source = "local-fallback"
    }

    return ValidateOutput{
        OK:      /* ok logic with observe mode */,
        EventID: res.EvidenceID,
        Source:  source,
        Policy: PolicySummary{
            Allow:     res.Pass,
            RiskLevel: res.RiskLevel,
            Reason:    firstReason(res.Reasons),
            PolicyRef: s.policyRef,
        },
        RuleIDs:   res.RuleIDs,
        Hints:     res.Hints,
        Reasons:   res.Reasons,
        Resources: s.resourceLinks(res.EvidenceID),
    }
}
```

Add `Source` to `ValidateOutput`:

```go
type ValidateOutput struct {
    OK        bool                    `json:"ok"`
    EventID   string                  `json:"event_id,omitempty"`
    Source    string                  `json:"source"`
    Policy    PolicySummary           `json:"policy"`
    RuleIDs   []string                `json:"rule_ids,omitempty"`
    Hints     []string                `json:"hints,omitempty"`
    Reasons   []string                `json:"reasons,omitempty"`
    Resources []evidence.ResourceLink `json:"resources,omitempty"`
    Error     *ErrorSummary           `json:"error,omitempty"`
}
```

---

## 10. File Changes Summary

### New files

| File | Purpose |
|---|---|
| `pkg/client/client.go` | HTTP client: `New()`, `Validate()`, `Ping()`, private `apiValidateResponse` |
| `pkg/client/client_test.go` | Unit tests with `httptest.Server` |
| `pkg/client/errors.go` | Sentinel errors + `IsReachabilityError()` |
| `pkg/mode/mode.go` | Mode resolution: `Resolve()` (no I/O, instant) |
| `pkg/mode/mode_test.go` | Unit tests (pure, no httptest needed) |

### Modified files

| File | Changes |
|---|---|
| `pkg/validate/validate.go` | Add `Source`, `PolicyRef` to `Result`. Set `Source: "local"` |
| `pkg/evidence/types.go` | Add `Source string` to `EvidenceRecord` |
| `pkg/config/config.go` | Add `ResolveURL()`, `ResolveAPIKey()`, `NormalizeEnvironment()` |
| `cmd/evidra/main.go` | New flags, mode resolution, online/fallback paths, exit code 3, JSON error |
| `cmd/evidra-mcp/main.go` | New flags, mode resolution, conditional bundle extraction |
| `pkg/mcpserver/server.go` | `APIClient`, `FallbackPolicy`, `IsOnline` in Options/Service. Online path. `Source` in output |

### NOT modified

| File | Reason |
|---|---|
| `pkg/runtime/runtime.go` | Pure OPA evaluation |
| `pkg/policy/policy.go` | OPA wrapper |
| `pkg/scenario/load.go` | Scenario loading |
| `pkg/bundlesource/` | Bundle source |
| `pkg/evidence/evidence.go` | Local evidence store (still used offline/fallback) |
| `cmd/evidra/evidence_cmd.go` | Evidence commands stay local-only |
| `cmd/evidra/policy_sim_cmd.go` | Policy sim stays local-only |

---

## 11. Testing Strategy

### Unit tests

**`pkg/client/client_test.go`:**
- `TestValidate_Success`: mock 200 + allow=true → Result{Source:"api"}
- `TestValidate_Denied`: mock 200 + allow=false → Result{Pass:false, Source:"api"}
- `TestValidate_Unauthorized`: mock 401 → ErrUnauthorized, `IsReachabilityError()==false`
- `TestValidate_ServerError`: mock 500 → ErrServerError, `IsReachabilityError()==true`
- `TestValidate_Unreachable`: dead port → ErrUnreachable, `IsReachabilityError()==true`
- `TestValidate_RateLimited`: mock 429 → ErrRateLimited, `IsReachabilityError()==false`
- `TestValidate_InvalidInput`: mock 422 → ErrInvalidInput, `IsReachabilityError()==false`
- `TestValidate_UnknownFields`: mock 200 with extra fields → silently ignored
- `TestPing_Success`: mock /healthz 200 → nil
- `TestPing_Unreachable`: dead port → error

**`pkg/mode/mode_test.go`:**
- `TestResolve_NoURL`: URL empty → Offline, Client nil
- `TestResolve_ForceOffline`: URL set + ForceOffline → Offline, Client nil
- `TestResolve_NoAPIKey`: URL set, no key → error
- `TestResolve_Online`: URL + key → Online, Client non-nil, **no I/O happened**
- `TestResolve_FallbackNormalization`: "OFFLINE" → "offline", "" → "closed", "junk" → "closed"

### Integration tests

**`cmd/evidra/main_test.go`:**
- `TestValidate_OnlineMode`: httptest server, EVIDRA_URL → Source="api"
- `TestValidate_OfflineFlag`: EVIDRA_URL + --offline → Source="local"
- `TestValidate_FallbackClosed`: dead port → exit code 3
- `TestValidate_FallbackClosed_JSON`: dead port + --json → error.code="API_UNREACHABLE", mode="online", url present
- `TestValidate_FallbackOffline`: dead port + EVIDRA_FALLBACK=offline → Source="local-fallback"
- `TestValidate_AuthError`: mock 401 → exit code 1 (not 3, no fallback)
- `TestValidate_RateLimit_NoFallback`: mock 429 + fallback=offline → exit code 1 (not local eval)
- `TestValidate_Denied_ExitCode2`: mock 200 + allow=false → exit code 2 (not 1)
- `TestValidate_UsageError_ExitCode4`: missing file → exit code 4
- `TestValidate_EnvNormalization`: EVIDRA_ENVIRONMENT=prod → server receives environment="production"

**Contract tests (`pkg/invocation/` or `cmd/evidra/`):**
- `TestActionToInvocation_KeysMatchAllowlist`: build invocation with all keys from `actionToInvocation()`, assert `ValidateStructure()` passes
- `TestActionToInvocation_RoundTrip`: scenario → invocation → validate matches expected input shape

**`cmd/evidra-mcp/test/`:**
- `TestMCP_OnlineMode`: httptest, EVIDRA_URL → Source="api"
- `TestMCP_OfflineMode`: no EVIDRA_URL → Source="local"
- `TestMCP_FallbackOffline`: dead port + EVIDRA_FALLBACK=offline → Source="local-fallback"

### Existing tests

All existing tests pass unchanged — they don't set `EVIDRA_URL`, so they resolve to Offline.

---

## 12. Implementation Steps (for Claude Code)

### Step 1: Add `Source`, `PolicyRef`, `RequestIDs` to `validate.Result`

**Files:** `pkg/validate/validate.go`, `pkg/evidence/types.go`

Add fields. In `EvaluateScenario`, set:
- `Source: "local"`
- `PolicyRef`: derive from bundle metadata or policy path. For embedded bundle: `"ops-v0.1"`. For loose policy: `"loose:<filename>"`. For custom bundle: `"bundle:<path>"`. This ensures JSON output is symmetric between online and offline — `PolicyRef` is never empty.

Also add `Source string` to `EvidenceRecord`.

**Acceptance:** `go test ./pkg/validate/... ./pkg/evidence/...` passes. Verify `PolicyRef` is populated in existing test outputs (not empty string).

### Step 2: Create `pkg/client`

**Files:** `pkg/client/client.go`, `pkg/client/errors.go`, `pkg/client/client_test.go`

Implement §4. Key: `New()` does no I/O. `apiValidateResponse` is private. stdlib only.

**Acceptance:** `go test -race ./pkg/client/...` — all pass.

### Step 3: Create `pkg/mode`

**Files:** `pkg/mode/mode.go`, `pkg/mode/mode_test.go`

Implement §5. Key: `Resolve()` does no I/O. Pure config validation.

**Acceptance:** `go test -race ./pkg/mode/...` — all pass.

### Step 4: Update CLI

**File:** `cmd/evidra/main.go`

Add flags (`--timeout` included), mode resolution, `evaluateOnline()`, `evaluateLocal()`, `actionToInvocation()`, exit codes 2/3/4, JSON error with `mode`/`url`/`request_id`. §7.

**Critical:** `actionToInvocation()` keys (`target`, `payload`, `risk_tags`, `intent`) MUST match `invocation.ValidateStructure()` allowlist. Add a test `TestActionToInvocation_KeysMatchAllowlist` that asserts every key used is in `allowedParamKeys`. This prevents "silent allow" where a param is sent but ignored by the policy because the key doesn't match the input mapping.

**Acceptance:** Existing + new tests pass. `go build` succeeds. Key allowlist test passes.

### Step 5: Update MCP server

**Files:** `cmd/evidra-mcp/main.go`, `pkg/mcpserver/server.go`

Add flags, mode resolution, conditional bundle extraction, online path in Validate(). §8 + §9.

**Acceptance:** Existing + new tests pass. `go build` succeeds.

### Step 6: Update `pkg/config`

Add `ResolveURL()`, `ResolveAPIKey()`, and `NormalizeEnvironment()`.

```go
// NormalizeEnvironment canonicalizes environment names to prevent
// silent policy mismatches (e.g. "prod" matching no by_env rules).
func NormalizeEnvironment(env string) string {
    v := strings.ToLower(strings.TrimSpace(env))
    switch v {
    case "prod", "prd":
        return "production"
    case "stg", "stage":
        return "staging"
    // NOTE: "dev" is NOT expanded to "development" — existing by_env overrides
    // commonly use "dev" as the canonical name. Expanding would silently disable
    // overrides unless data.json also lists "development".
    default:
        return v // unknown values (including "dev") pass through unchanged
    }
}
```

Call `NormalizeEnvironment()` in CLI and MCP when resolving the `--environment` flag / `EVIDRA_ENVIRONMENT` env var — before passing to `validate.Options` or `client.Validate()`.

**Acceptance:** `go test ./pkg/config/...` passes. Tests cover: `"prod"→"production"`, `"PRD"→"production"`, `"staging"→"staging"`, `"dev"→"dev"` (unchanged), `"custom"→"custom"`, `""→""`.

### Step 7: Update CLAUDE.md

Add hybrid mode section, new env vars, exit code 3.

---

## 13. Checklist Before Merge

```
[ ] go test -race -cover ./... — all pass
[ ] go vet ./... — clean
[ ] gofmt -d . — no diff
[ ] go mod tidy — no diff
[ ] Existing tests pass unchanged (offline is default)
[ ] pkg/client: 10+ tests, httptest, stdlib only, no ping on hot path
[ ] pkg/client: X-Request-ID header on every POST, returned to caller
[ ] pkg/client: classifyError handles timeout, canceled, net.Error, all HTTP codes
[ ] pkg/client: 429 → ErrRateLimited, IsReachabilityError()==false (never triggers fallback)
[ ] pkg/mode: 5+ tests, pure (zero I/O in Resolve)
[ ] CLI: online, offline, fallback-closed (exit 3), fallback-offline, auth error
[ ] CLI: exit 2 = denied, 3 = unreachable, 4 = usage (distinct codes, tested)
[ ] MCP: online, offline, fallback-offline
[ ] MCP: embedded bundle cached to ~/.evidra/bundles/ (not tmpdir)
[ ] Source field in Result, EvidenceRecord, ValidateOutput
[ ] EvidenceIDs []string collected for multi-action scenarios
[ ] RiskLevel aggregated as max(low < medium < high < critical)
[ ] RuleIDs and Hints deduped in multi-action aggregation
[ ] PolicyRef drift: "mixed" if server returns different refs across actions
[ ] JSON output includes mode, request_id, error.url
[ ] NormalizeEnvironment: prod→production, stg→staging (dev stays as-is)
[ ] actionToInvocation keys match ValidateStructure allowlist (test assertion)
[ ] CLI always converts Scenario → ToolInvocation (no raw passthrough)
[ ] apiValidateResponse is private, unknown fields ignored
[ ] --timeout flag plumbs to client.Config.Timeout
[ ] --verbose logs request IDs, timing, policy drift warnings to stderr
[ ] CLAUDE.md updated
[ ] evidence and policy sim subcommands unchanged
```

---

## 14. What This Does NOT Include

- **`evidra-api` changes**: Unchanged. Already has `/v1/validate` and `/healthz`.
- **Evidence sync**: Phase 2. Requires new API endpoint.
- **Skills in CLI**: Phase 2.
- **`get_event` online**: Needs API endpoint.
- **`evidra doctor`**: Future. Would use `client.Ping()`.
- **Auth token rotation**: Out of scope.
