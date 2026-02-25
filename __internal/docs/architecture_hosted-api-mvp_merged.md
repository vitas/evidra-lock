# Evidra Hosted API — MVP Architecture

**Date:** 2026-02-24
**Status:** Draft
**Scope:** Minimal hosted endpoint wrapping the existing Evidra engine

---

## 1. High-Level Architecture

```
                          ┌──────────────------────┐
                          │  https://evidra.rest   │
                          │  (static HTML)         │
                          │                        │
                          │  [Get API Key]───------┼--──── POST /v1/keys
                          └─────────────────------─┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Reverse Proxy (TLS)                        │
│                   nginx / Trafik / Caddy /                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│                       evidra-api (Go)                          │
│                                                                │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌──────────────┐   │
│  │ Router   │→ │ Auth MW  │→ │ Handlers  │→ │ Engine       │   │
│  │ (stdlib) │  │ (Bearer) │  │           │  │ Adapter      │   │
│  └──────────┘  └──────────┘  └─────┬─────┘  └──────┬───────┘   │
│                                    │                │          │
│                              ┌─────▼─────┐   ┌─────▼───────-┐  │
│                              │ Storage   │   │ pkg/validate │  │
│                              │ (Postgres)│   │ pkg/runtime  │  │
│                              └─────┬─────┘   │ pkg/policy   │  │
│                                    │         └──────────────┘  │
└────────────────────────────────────┼───────────────────────────┘
                                     │
                                     ▼
                          ┌──────────────────┐
                          │   PostgreSQL     │
                          │                  │
                          │ tenants          │
                          │ api_keys         │
                          │ evidence         │
                          │ usage_counters   │
                          └──────────────────┘
```

### Components

**Landing page** — A single static HTML file served by the reverse proxy (or
embedded in the Go binary via `embed`). Contains one form field (optional label)
and a "Get API Key" button that POSTs to `/v1/keys`. The response displays the
key exactly once.

**API service (`evidra-api`)** — A single Go binary. Stateless except for an
in-memory OPA engine (loaded once at startup from the bundled policy profile).
All durable state lives in Postgres.

**PostgreSQL** — Single instance. Stores tenants, hashed API keys, evidence
records, and usage counters. No read replicas for MVP.

**Evidence storage: Postgres JSONB (not filesystem/S3).**
Justification: The existing JSONL store is designed for single-machine append
with file locking — it cannot serve concurrent tenants on a shared host without
per-tenant directories and process-level coordination. Postgres gives us tenant
isolation via `WHERE tenant_id = $1`, transactional appends, indexed lookups by
`evidence_id`, and hash-chain validation via window functions — all without
introducing a second storage system. At MVP scale (thousands of records/day),
JSONB columns are more than sufficient. If evidence volume grows past what
Postgres handles comfortably, the structured columns make migration to S3 + a
metadata table straightforward.

### Request Flows

**A) Issue Key Flow**
```
Browser → POST /v1/keys {label?}
  1. Generate tenant_id (ULID)
  2. Generate API key: "ev1_" + 32 crypto/rand bytes (base62), 48 chars total
  3. Hash key with SHA-256 (see §4 for rationale)
  4. INSERT into tenants (id, created_at)
  5. INSERT into api_keys (id, tenant_id, key_hash, prefix, label, created_at)
  6. Return {key: "ev1_...", prefix: "ev1_...<first 8>", tenant_id} — plaintext shown once
```

**B) Validate Flow**
```
Agent → POST /v1/validate  [Authorization: Bearer ev1_...]
  1. Auth middleware: SHA-256(key), lookup api_keys WHERE key_hash = $1
     → reject if not found or revoked_at IS NOT NULL
     → set tenant_id in request context
  2. Handler: unmarshal body as invocation.ToolInvocation
  3. Adapter: call validate.EvaluateInvocation(ctx, inv, opts)
     where opts.SkipEvidence = true (we write evidence to Postgres, not JSONL)
  4. Build evidence record, INSERT into evidence table with tenant_id
  5. Increment usage counter (upsert into usage_counters)
  6. Update api_keys SET last_used_at = now()
  7. Return {ok, event_id, policy: {allow, risk_level, reason, policy_ref},
            rule_ids, hints, reasons}
```

**C) Evidence Retrieval Flow**
```
Agent → GET /v1/evidence/{evidence_id}  [Authorization: Bearer ev1_...]
  1. Auth middleware → tenant_id
  2. SELECT FROM evidence WHERE evidence_id = $1 AND tenant_id = $2
     → 404 if not found (never leak existence across tenants)
  3. Return evidence record JSON
```

**Tenant identity** — Derived entirely from the API key. Each key belongs to
exactly one tenant. No separate authentication. The `tenant_id` is set in
`context.Context` by the auth middleware and threaded through to all storage
calls.

---

## 2. Minimal API Surface

### `POST /v1/keys`

No authentication required (public endpoint, rate-limited by IP).

**Request:**
```json
{"label": "my-ci-pipeline"}
```
`label` is optional, max 128 chars. Empty body is valid.

**Response (201):**
```json
{
  "key": "ev1_a8Fk3mQ9x...48chars",
  "prefix": "ev1_a8Fk3mQ9",
  "tenant_id": "01JEXAMPLE..."
}
```
The plaintext `key` is returned exactly once. It is never stored or retrievable.

**Errors:**
- `429` — rate limit exceeded
- `400` — label too long

---

### `POST /v1/validate`

Requires `Authorization: Bearer <key>`.

**Request body** — identical to `invocation.ToolInvocation`:
```json
{
  "actor":     {"type": "agent", "id": "deploy-bot", "origin": "api"},
  "tool":      "terraform",
  "operation": "apply",
  "params": {
    "target":    {"namespace": "prod", "cluster": "us-east-1"},
    "payload":   {"resource_count": 12},
    "risk_tags": ["change-approved"]
  },
  "context": {
    "source": "ci-pipeline",
    "intent": "deploy v2.3"
  }
}
```

No new schema. The body is deserialized directly into `invocation.ToolInvocation`.

**Optional query parameter:** `?profile=ops-v0.1` (reserved for future multi-profile support; MVP uses the single bundled profile).

**Response (200):**
```json
{
  "ok": true,
  "event_id": "evt-1708789200000000000",
  "policy": {
    "allow": true,
    "risk_level": "low",
    "reason": "",
    "policy_ref": "sha256:abc123..."
  },
  "rule_ids": [],
  "hints": [],
  "reasons": []
}
```

The response shape matches `mcpserver.ValidateOutput` (minus MCP-specific
`resources` field). This keeps client code portable between MCP and HTTP.

**Errors:**
- `400` — invalid input (fails `ValidateStructure`)
- `401` — missing or invalid key
- `422` — policy evaluation error
- `429` — rate limit exceeded
- `500` — evidence write failure or internal error

Error body:
```json
{
  "ok": false,
  "error": {"code": "invalid_input", "message": "actor.type is required"}
}
```

---

### `GET /v1/evidence/{evidence_id}`

Requires `Authorization: Bearer <key>`.

**Response (200):**
```json
{
  "ok": true,
  "record": {
    "event_id": "evt-...",
    "timestamp": "2026-02-24T10:00:00Z",
    "tenant_id": "01J...",
    "policy_ref": "sha256:...",
    "actor": {"type": "agent", "id": "deploy-bot", "origin": "api"},
    "tool": "terraform",
    "operation": "apply",
    "policy_decision": {
      "allow": true,
      "risk_level": "low",
      "reason": "",
      "reasons": [],
      "hints": [],
      "rule_ids": []
    },
    "input_hash": "sha256:...",
    "chain_prev_hash": "sha256:...",
    "chain_hash": "sha256:..."
  }
}
```

**Errors:**
- `401` — missing or invalid key
- `404` — not found (or belongs to a different tenant — same response)

---

### `GET /healthz`

No auth. Returns `200 OK` with `{"status": "ok"}`. Checks that the process is
running and the OPA engine is loaded.

### `GET /readyz`

No auth. Returns `200 OK` only when Postgres is reachable (a `SELECT 1` probe).
Returns `503` otherwise. Used by orchestrators for readiness gating.

---

## 3. Data Model (PostgreSQL)

### `tenants`

Auto-created when a key is issued. One tenant per API key for MVP (1:1). The
table exists so that future features (multiple keys per tenant, revocation
scopes) have a stable foreign key target.

```sql
CREATE TABLE tenants (
    id          TEXT        PRIMARY KEY,          -- ULID
    label       TEXT        NOT NULL DEFAULT '',   -- optional human label
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

### `api_keys`

Stores only the SHA-256 hash of the key, never the plaintext.

```sql
CREATE TABLE api_keys (
    id           TEXT        PRIMARY KEY,          -- ULID
    tenant_id    TEXT        NOT NULL REFERENCES tenants(id),
    key_hash     BYTEA       NOT NULL,             -- SHA-256(plaintext key), 32 bytes
    prefix       TEXT        NOT NULL,             -- "ev1_<first8>" for log correlation
    label        TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,                      -- NULL = active
    last_used_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_api_keys_hash ON api_keys (key_hash);
CREATE INDEX idx_api_keys_tenant ON api_keys (tenant_id);
```

**Why SHA-256 instead of Argon2id/bcrypt:** API keys are high-entropy secrets
(32 random bytes = 256 bits). Unlike user passwords, they are not guessable or
dictionary-attackable. SHA-256 is the industry standard for hashing
high-entropy API keys (used by Stripe, GitHub, AWS). It enables O(1) lookup
via index, whereas bcrypt/Argon2id require a scan or a separate lookup column
and add ~100ms per authentication. For MVP with rate-limited key issuance,
SHA-256 is the correct choice.

---

### `evidence`

Each row is one evaluation record, tenant-scoped.

```sql
CREATE TABLE evidence (
    id              TEXT        PRIMARY KEY,          -- "evt-<UnixNano>"
    tenant_id       TEXT        NOT NULL REFERENCES tenants(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    policy_ref      TEXT        NOT NULL,
    actor_type      TEXT        NOT NULL,
    actor_id        TEXT        NOT NULL,
    actor_origin    TEXT        NOT NULL,
    tool            TEXT        NOT NULL,
    operation       TEXT        NOT NULL,
    decision_allow  BOOLEAN     NOT NULL,
    decision_risk   TEXT        NOT NULL,             -- "low" | "medium" | "high"
    decision_reason TEXT        NOT NULL DEFAULT '',
    rule_ids        TEXT[]      NOT NULL DEFAULT '{}',
    hints           TEXT[]      NOT NULL DEFAULT '{}',
    reasons         TEXT[]      NOT NULL DEFAULT '{}',
    input_hash      TEXT        NOT NULL,             -- SHA-256 of serialized input
    chain_prev_hash TEXT        NOT NULL DEFAULT '',   -- hash of previous record for this tenant
    chain_hash      TEXT        NOT NULL,             -- hash of this record
    params_jsonb    JSONB,                            -- full input params (for audit replay)
    context_jsonb   JSONB                             -- full input context
);

CREATE INDEX idx_evidence_tenant_created ON evidence (tenant_id, created_at DESC);
CREATE INDEX idx_evidence_tenant_id ON evidence (tenant_id, id);
```

The `chain_prev_hash` / `chain_hash` columns preserve the hash-linked chain
property from the JSONL store, scoped per tenant. The chain is validated on
read by querying records in `created_at` order and verifying hashes.

---

### `usage_counters`

Pre-aggregated counters in 1-hour buckets. No raw request logs for MVP — this
keeps storage bounded and avoids PII concerns.

```sql
CREATE TABLE usage_counters (
    tenant_id       TEXT        NOT NULL REFERENCES tenants(id),
    endpoint        TEXT        NOT NULL,             -- "validate" | "evidence" | "keys"
    bucket          TIMESTAMPTZ NOT NULL,             -- truncated to hour
    request_count   BIGINT      NOT NULL DEFAULT 0,
    allow_count     BIGINT      NOT NULL DEFAULT 0,   -- only for "validate"
    deny_count      BIGINT      NOT NULL DEFAULT 0,   -- only for "validate"
    error_count     BIGINT      NOT NULL DEFAULT 0,
    latency_sum_ms  BIGINT      NOT NULL DEFAULT 0,   -- for avg = sum / count
    latency_max_ms  INT         NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, endpoint, bucket)
);
```

Upsert on every request:
```sql
INSERT INTO usage_counters (tenant_id, endpoint, bucket, request_count,
    allow_count, deny_count, error_count, latency_sum_ms, latency_max_ms)
VALUES ($1, $2, date_trunc('hour', now()), 1, $3, $4, $5, $6, $7)
ON CONFLICT (tenant_id, endpoint, bucket) DO UPDATE SET
    request_count  = usage_counters.request_count + 1,
    allow_count    = usage_counters.allow_count + EXCLUDED.allow_count,
    deny_count     = usage_counters.deny_count + EXCLUDED.deny_count,
    error_count    = usage_counters.error_count + EXCLUDED.error_count,
    latency_sum_ms = usage_counters.latency_sum_ms + EXCLUDED.latency_sum_ms,
    latency_max_ms = GREATEST(usage_counters.latency_max_ms, EXCLUDED.latency_max_ms);
```

**Retention:** A daily cron or pg_cron job deletes rows older than 90 days:
```sql
DELETE FROM usage_counters WHERE bucket < now() - INTERVAL '90 days';
```

---

## 4. Security Design

### API Key Format

```
ev1_<32 random bytes encoded as base62>
```

- **Prefix `ev1_`** — identifies the key version. Enables grep in logs, rotation
  of key format without breaking parsing, and visual identification.
- **32 random bytes** — 256 bits of entropy from `crypto/rand`. Encoded as
  base62 (a-zA-Z0-9) producing ~43 characters. Total key length: ~47 chars.
- **Prefix stored separately** — the first 12 characters (e.g. `ev1_a8Fk3mQ9`)
  are stored in the `prefix` column for log correlation without exposing the
  full key.

### Hashing Strategy

**SHA-256** for API key hashing (see justification in §3 under `api_keys`).

```go
hash := sha256.Sum256([]byte(plaintextKey))
// Store hash[:] as BYTEA in Postgres
```

No salt needed — the key itself has 256 bits of entropy.

### Constant-Time Comparison

Authentication performs:
1. SHA-256 hash the incoming Bearer token.
2. Query Postgres by `key_hash`.
3. If no row found, return 401 after a constant-time sleep (50-100ms jitter)
   to prevent timing-based enumeration.
4. `crypto/subtle.ConstantTimeCompare` is not needed for the hash lookup
   itself (Postgres index lookup), but is used if any secondary comparison is
   done in application code.

### Safe Parsing

```go
func parseAPIKey(header string) (string, error) {
    // Require "Bearer " prefix (case-sensitive, single space)
    // Require key starts with "ev1_"
    // Require total key length between 40 and 60 chars
    // Reject keys containing non-base62 characters after prefix
    // Return sanitized key string
}
```

### "Show Once" Behavior

1. The plaintext key is generated in memory, hashed, and stored.
2. The plaintext is returned in the HTTP response body.
3. The plaintext is never written to Postgres, logs, or any durable store.
4. The response includes `Cache-Control: no-store` to prevent browser caching.
5. Server logs record only the `prefix` field, never the full key.

### Rate Limiting

**Per-key limits:**
- `/v1/validate`: 60 requests/minute (token bucket)
- `/v1/evidence/*`: 120 requests/minute
- Burst allowance: 2x the per-minute rate

**Per-IP limits (unauthenticated):**
- `POST /v1/keys`: 5 requests/hour per IP
- Global: 100 requests/minute per IP across all endpoints

**Implementation:** In-memory token bucket (`golang.org/x/time/rate`) keyed by
tenant_id (authenticated) or IP (unauthenticated). Sufficient for single-node
MVP. Returns `429 Too Many Requests` with `Retry-After` header.

### Abuse Mitigations

- **IP throttling** on key issuance prevents key farming.
- **Key revocation:** `api_keys.revoked_at` — set via internal admin query for
  MVP (no admin API yet). Auth middleware checks `revoked_at IS NULL`.
- **Request size limit:** 1 MB max body on all endpoints. Enforced by
  `http.MaxBytesReader`.
- **Idle key cleanup:** Keys not used in 90 days are candidates for revocation
  (manual review for MVP; automated later).

### Tenant Isolation

Every storage query includes `WHERE tenant_id = $1`. The `tenant_id` is
extracted from the authenticated API key in middleware and injected into
`context.Context`. Handlers never accept tenant_id from request parameters.

Evidence retrieval returns 404 (not 403) for records belonging to other tenants,
preventing existence enumeration.

### Logging / Redaction Rules

- **Never log:** plaintext API keys, request body `params` or `context` contents
  (may contain infrastructure details).
- **Always log:** key prefix, tenant_id, endpoint, status code, latency,
  evidence_id, decision (allow/deny/risk_level).
- **Structured JSON logging** via `log/slog` with redaction enforced at the
  logger level — sensitive fields are not passed to the logger in the first
  place (not filtered after the fact).

---

## 5. Code Architecture (Go)

```
cmd/
  evidra-api/
    main.go                  # Entrypoint: parse env, init DB, init engine, start server

internal/
  api/
    router.go                # stdlib http.ServeMux, mounts handlers
    middleware.go            # Request logging, recovery, request-id, CORS, body limit
    keys_handler.go          # POST /v1/keys
    validate_handler.go      # POST /v1/validate
    evidence_handler.go      # GET /v1/evidence/{id}
    health_handler.go        # GET /healthz, /readyz
    response.go              # JSON response helpers, error formatting

  auth/
    middleware.go            # Bearer token extraction, key lookup, tenant context injection
    apikey.go                # Key generation, parsing, hashing functions
    context.go               # TenantID get/set on context.Context

  engine/
    adapter.go               # Thin wrapper: calls pkg/validate.EvaluateInvocation
                             # Converts validate.Result → internal result type
                             # Manages OPA engine lifecycle (init once at startup)

  storage/
    postgres.go              # *sql.DB initialization, connection pool, ping
    tenants.go               # TenantRepo: Create
    apikeys.go               # APIKeyRepo: Create, FindByHash, Revoke, TouchLastUsed
    evidence.go              # EvidenceRepo: Insert, FindByID (tenant-scoped), LastHash
    usage.go                 # UsageRepo: Increment (upsert usage_counters)

  ratelimit/
    limiter.go               # Token bucket per key/IP, cleanup goroutine

  migrate/
    migrations/
      001_initial.up.sql     # CREATE TABLE tenants, api_keys, evidence, usage_counters
      001_initial.down.sql

pkg/                         # EXISTING — unchanged
  validate/                  # EvaluateInvocation, EvaluateScenario
  invocation/                # ToolInvocation, ValidateStructure
  policy/                    # OPA engine
  runtime/                   # Evaluator, PolicySource
  policysource/              # LocalFileSource
  evidence/                  # JSONL store (still used by CLI/MCP; NOT used by API)
  config/                    # Path resolution
  scenario/                  # Scenario schema
```

### Dependency Direction

```
cmd/evidra-api
  → internal/api        (HTTP layer)
  → internal/auth       (middleware)
  → internal/engine     (adapter)
  → internal/storage    (repos)
  → internal/ratelimit

internal/api
  → internal/auth       (extracts tenant from context)
  → internal/engine     (calls Evaluate)
  → internal/storage    (reads/writes evidence, keys, usage)
  → internal/ratelimit

internal/auth
  → internal/storage    (looks up key hash)

internal/engine
  → pkg/validate        (calls EvaluateInvocation)
  → pkg/invocation      (ToolInvocation type)

internal/storage
  → (database/sql only; no pkg/ imports)

pkg/*
  → (no internal/ imports — engine is unaware of HTTP or Postgres)
```

**Key boundary:** `internal/engine` is the only package that imports `pkg/*`.
It is a thin adapter — no business logic, no schema translation beyond what
`validate.EvaluateInvocation` already does.

---

## 6. Reuse of Existing Engine

### Entry point

The API calls exactly one function:

```go
validate.EvaluateInvocation(ctx, inv, opts)
```

where:
- `inv` is the `invocation.ToolInvocation` deserialized directly from the HTTP
  request body (no transformation needed — the API accepts the same schema).
- `opts` is:
  ```go
  validate.Options{
      PolicyPath:   "<bundled profile path>",
      DataPath:     "<bundled data.json path>",
      SkipEvidence: true,  // API writes evidence to Postgres, not JSONL
  }
  ```

### Profile selection

MVP bundles the single `ops-v0.1` profile. The `PolicyPath` and `DataPath` are
resolved at startup (either embedded in the binary or read from a config path).
The `?profile=` query parameter is accepted but ignored for now — it exists so
clients can start passing it without breaking when multi-profile is added.

To keep startup fast and avoid re-reading files on every request, the adapter
initializes a `runtime.Evaluator` once:

```go
type Adapter struct {
    evaluator *runtime.Evaluator
    policyRef string
}

func NewAdapter(policyPath, dataPath string) (*Adapter, error) {
    src := policysource.NewLocalFileSource(policyPath, dataPath)
    eval, err := runtime.NewEvaluator(src)
    // ...
    ref, _ := src.PolicyRef()
    return &Adapter{evaluator: eval, policyRef: ref}, nil
}

func (a *Adapter) Evaluate(ctx context.Context, inv invocation.ToolInvocation) (Result, error) {
    if err := inv.ValidateStructure(); err != nil {
        return Result{}, err
    }
    decision, err := a.evaluator.EvaluateInvocation(inv)
    // ... map decision to Result
}
```

This bypasses `validate.EvaluateScenario` (which handles file loading and JSONL
evidence) and calls the runtime evaluator directly. The adapter performs
`ValidateStructure` itself since the runtime no longer does (per TD-07).

### Evidence linking to tenant

After `Evaluate` returns, the handler builds an evidence row:

```go
rec := storage.EvidenceRow{
    ID:             fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano()),
    TenantID:       auth.TenantFromContext(ctx),
    PolicyRef:      decision.PolicyRef,
    Actor:          inv.Actor,
    Tool:           inv.Tool,
    Operation:      inv.Operation,
    DecisionAllow:  decision.Allow,
    DecisionRisk:   decision.RiskLevel,
    DecisionReason: decision.Reason,
    RuleIDs:        decision.Hits,
    Hints:          decision.Hints,
    Reasons:        decision.Reasons,
    InputHash:      sha256OfInput(inv),
    ParamsJSONB:    inv.Params,
    ContextJSONB:   inv.Context,
}
// chain_prev_hash and chain_hash computed by EvidenceRepo.Insert
```

The repo's `Insert` method queries the previous record's `chain_hash` for this
tenant (or `""` if first record), computes the new hash, and inserts atomically
within a transaction.

---

## 7. Deployment Plan (MVP)

### Artifacts

- **Single binary:** `go build -o bin/evidra-api ./cmd/evidra-api`
- **Policy bundle:** The `policy/bundles/ops-v0.1/` directory is either
  embedded via `//go:embed` or mounted as a volume. Embedding is preferred for
  simplicity.
- **Migrations:** SQL files in `internal/migrate/migrations/`, applied at
  startup or via CLI flag.

### Configuration (env vars)

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres connection string |
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address |
| `EVIDRA_API_POLICY_PATH` | no | embedded | Override policy .rego path |
| `EVIDRA_API_DATA_PATH` | no | embedded | Override data.json path |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | no | `json` | `json` or `text` |

### TLS

TLS termination is handled by the reverse proxy (nginx, Caddy, or cloud LB).
The Go binary listens on plain HTTP. If no reverse proxy is available,
`LISTEN_TLS_CERT` and `LISTEN_TLS_KEY` env vars enable direct TLS (stdlib
`http.ListenAndServeTLS`).

### Migrations

Use **golang-migrate** (`github.com/golang-migrate/migrate/v4`):
- Migrations are embedded in the binary via `//go:embed`.
- On startup, if `--migrate` flag is set, run migrations before starting the
  server. Otherwise, check that the schema is at the expected version and fail
  fast if not.
- For production: run migrations as a separate step in the deploy pipeline
  (`evidra-api migrate up`).

### Observability

**Structured logging:** `log/slog` with JSON output. Every request logs:
`request_id`, `method`, `path`, `status`, `latency_ms`, `tenant_id` (if
authenticated), `key_prefix`.

**Prometheus metrics (optional but minimal):**

```
evidra_api_requests_total{endpoint, status_code}        counter
evidra_api_request_duration_seconds{endpoint}            histogram
evidra_api_decisions_total{allow, risk_level}             counter
evidra_api_keys_issued_total                              counter
evidra_api_active_connections                              gauge
```

Exposed at `GET /metrics` (behind a separate port or basic auth for MVP).

### Process management

- Graceful shutdown on SIGTERM (drain in-flight requests, close DB pool).
- Health checks: `/healthz` (liveness), `/readyz` (readiness — DB reachable).
- Run as a systemd service, Docker container, or on a PaaS (Fly.io, Railway).

---

## 8. Traffic Measurement Plan

### Metrics to Evaluate Demand

| Metric | Source | Query |
|---|---|---|
| **Keys issued (total)** | `tenants` table | `SELECT count(*) FROM tenants` |
| **Keys issued (last 7d)** | `tenants` table | `WHERE created_at > now() - '7d'` |
| **Active keys (used in last 7d)** | `api_keys` table | `WHERE last_used_at > now() - '7d'` |
| **Request rate per endpoint** | `usage_counters` | `SUM(request_count) GROUP BY endpoint, bucket` |
| **Deny/allow ratio** | `usage_counters` | `SUM(deny_count) / SUM(allow_count) WHERE endpoint = 'validate'` |
| **Error rate** | `usage_counters` | `SUM(error_count) / SUM(request_count)` |
| **p95 latency** | Prometheus histogram | `histogram_quantile(0.95, ...)` |
| **Top tenants by volume** | `usage_counters` | `GROUP BY tenant_id ORDER BY SUM(request_count) DESC LIMIT 20` |

### Retention

| Data | Retention |
|---|---|
| `usage_counters` rows | 90 days (daily cleanup) |
| `evidence` rows | Indefinite (append-only audit trail) |
| Prometheus metrics | 14 days (default Prometheus retention) |
| Structured logs | 30 days (external log sink) |

### Demand signals (when to invest further)

- >50 active keys → add org/team support
- >10k requests/day → add async usage counter writes (batch insert)
- p95 latency >200ms → profile OPA evaluation, add engine caching
- >100 keys/week issuance → add email verification or abuse detection

---

## MVP Build Order

1. **Database schema and migrations**
   - Write `001_initial.up.sql` / `down.sql`
   - Set up golang-migrate with embedded SQL

2. **Storage layer (`internal/storage`)**
   - `postgres.go` — connection pool init
   - `tenants.go` — `Create`
   - `apikeys.go` — `Create`, `FindByHash`, `TouchLastUsed`
   - `evidence.go` — `Insert`, `FindByID`, `LastHashForTenant`
   - `usage.go` — `Increment`

3. **Auth (`internal/auth`)**
   - `apikey.go` — key generation, SHA-256 hashing, parsing
   - `context.go` — tenant context helpers
   - `middleware.go` — Bearer extraction, DB lookup, context injection

4. **Engine adapter (`internal/engine`)**
   - `adapter.go` — init `runtime.Evaluator` once, expose `Evaluate` method

5. **HTTP handlers (`internal/api`)**
   - `keys_handler.go` — POST /v1/keys
   - `validate_handler.go` — POST /v1/validate
   - `evidence_handler.go` — GET /v1/evidence/{id}
   - `health_handler.go` — /healthz, /readyz

6. **Rate limiter (`internal/ratelimit`)**
   - In-memory token bucket with cleanup

7. **Router and middleware (`internal/api`)**
   - `router.go` — mount all handlers
   - `middleware.go` — logging, recovery, request-id, body limit

8. **Entrypoint (`cmd/evidra-api`)**
   - Parse env vars, init DB, run migrations, init engine, start server

9. **Landing page**
   - Single HTML file with embedded JS for POST /v1/keys
   - Embed in binary or serve from reverse proxy

10. **Smoke test**
    - Issue a key, validate a request, retrieve evidence
    - Verify tenant isolation (key A cannot see key B's evidence)
