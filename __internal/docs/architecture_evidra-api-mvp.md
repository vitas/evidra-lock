# Evidra API — MVP Architecture

**Date:** 2026-02-25
**Status:** Draft
**Scope:** Unified hosted API: stateless policy evaluator with skills, client-side evidence
**Supersedes:** `architecture_hosted-api-mvp.md`, `architecture_skills-backend-mvp.md`

---

## 1. High-Level Architecture

<!-- CHANGED: Removed `evidence` table from Postgres. Added `skills` and `executions` tables.
     Removed `execution_evidence` join table. Added Evidence Signer component.
     The server is now a stateless policy evaluator — no long-term evidence storage. -->

```
                          ┌───────────────────────┐
                          │  https://evidra.rest   │
                          │  (static HTML)         │
                          │                        │
                          │  [Get API Key]─────────┼──── POST /v1/keys
                          └───────────────────────-┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Reverse Proxy (TLS)                            │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       evidra-api (Go)                               │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  ┌────────────┐  │
│  │ Router   │→ │ Auth MW  │→ │ Handlers         │→ │ Engine     │  │
│  │ (stdlib) │  │ (Bearer) │  │                  │  │ Adapter    │  │
│  └──────────┘  └──────────┘  │ keys_handler     │  └──────┬─────┘  │
│                              │ validate_handler  │         │        │
│                              │ skills_handler    │   ┌─────▼──────┐ │
│                              │ execute_handler   │   │ pkg/       │ │
│                              │ verify_handler    │   │ validate   │ │
│                              │ health_handler    │   │ runtime    │ │
│                              └────────┬─────────┘   │ policy     │ │
│                                       │              └────────────┘ │
│                           ┌───────────┼───────────┐                 │
│                           │           │           │                 │
│                     ┌─────▼─────┐ ┌───▼─────────┐│                 │
│                     │ Storage   │ │ Evidence    ││                 │
│                     │ (Postgres)│ │ Signer      ││                 │
│                     │           │ │ (Ed25519)   ││                 │
│                     │ tenants   │ └─────────────┘│                 │
│                     │ api_keys  │                 │                 │
│                     │ usage_ctr │                 │                 │
│                     │ skills    │                 │                 │
│                     │ executions│                 │                 │
│                     └─────┬─────┘                 │                 │
│                           │                       │                 │
└───────────────────────────┼───────────────────────┘─────────────────┘
                            │
                            ▼
                 ┌──────────────────┐
                 │   PostgreSQL     │
                 └──────────────────┘
```

### Components

**Landing page** — A single static HTML file served by the reverse proxy (or
embedded in the Go binary via `embed`). Contains one form field (optional label)
and a "Get API Key" button that POSTs to `/v1/keys`. The response displays the
key exactly once.

**API service (`evidra-api`)** — A single Go binary. Stateless except for an
in-memory OPA engine (loaded once at startup from the bundled policy profile)
and the Ed25519 signing key (loaded once at startup).
All durable state lives in Postgres.

<!-- CHANGED: Removed "evidence records" from Postgres description. Server is now
     explicitly a stateless policy evaluator. -->

**PostgreSQL** — Single instance. Stores tenants, hashed API keys, usage
counters, skill definitions, and lightweight execution records. No read replicas
for MVP. **No evidence records** — evidence is returned to the client in the
response body for client-side storage.

<!-- CHANGED: Replaced server-side evidence section. Added Evidence Signer component. -->

**Evidence Signer** — An Ed25519 signing module (`internal/evidence`). Every
`validate` and `execute` response includes a complete, server-signed
`evidence_record` that the client persists wherever they want. The server
publishes the public key at `GET /v1/evidence/pubkey` so anyone can verify a
record's authenticity without contacting the server.

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

<!-- CHANGED: Validate flow no longer writes to Postgres evidence table.
     Instead, builds an evidence record, signs it with Ed25519, and returns it
     in the response body. -->

**B) Validate Flow**
```
Agent → POST /v1/validate  [Authorization: Bearer ev1_...]
  1. Auth middleware: SHA-256(key), lookup api_keys WHERE key_hash = $1
     → reject if not found or revoked_at IS NOT NULL
     → set tenant_id in request context
  2. Handler: unmarshal body as invocation.ToolInvocation
  3. Adapter: call engine.Evaluate(ctx, inv) → Decision
  4. Build evidence record (event_id, timestamp, actor, decision, input_hash, ...)
  5. Build signing payload (deterministic text), sign with Ed25519
  6. Increment usage counter (upsert into usage_counters)
  7. Update api_keys SET last_used_at = now()
  8. Return {ok, event_id, decision, evidence_record (signed)}
```

<!-- CHANGED: Removed Evidence Retrieval Flow (C). Server has nothing to return.
     Added Evidence Verify Flow instead. -->

<!-- CHANGED(v2): Evidence Verify Flow is now public — no auth required.
     Removed tenant_id check (evidence records do not contain tenant_id). -->

**C) Evidence Verify Flow**
```
Anyone → POST /v1/evidence/verify  (no auth required, IP rate-limited)
  1. Unmarshal submitted evidence_record from request body
  2. Extract signing_payload and signature from record
  3. Verify Ed25519 signature over signing_payload
  4. Return {ok, valid: true/false, reason}
```

<!-- CHANGED: Added Skills Execute Flow showing no server-side evidence storage. -->

**D) Skills Execute Flow**
```
Agent → POST /v1/skills/{skill_id}:execute  [Authorization: Bearer ev1_...]
  1. Auth middleware → tenant_id
  2. Load skill definition (skills table, WHERE tenant_id AND skill_id)
  3. Validate input against skill.input_schema (JSON Schema draft-07)
  4. Build ToolInvocation from skill template + input
  5. Engine.Evaluate(ToolInvocation) → Decision
  6. Build evidence record, build signing payload, sign with Ed25519
  7. INSERT lightweight execution record (no evidence payload — just metadata)
  8. Increment usage_counters
  9. Return {ok, execution_id, event_id, decision, evidence_record (signed)}
```

**Tenant identity** — Derived entirely from the API key. Each key belongs to
exactly one tenant. No separate authentication. The `tenant_id` is set in
`context.Context` by the auth middleware and threaded through to all storage
calls.

---

## 2. API Surface

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

<!-- CHANGED: Response now includes `evidence_record` with server signature.
     No server-side evidence write. The `500 evidence write failure` error is
     removed — signing is in-memory and cannot fail in the DB-failure sense. -->

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

<!-- CHANGED(v2): evidence_record now includes `signing_payload` field instead
     of relying on canonical JSON serialization for signature verification. -->

**Response (200):**
```json
{
  "ok": true,
  "event_id": "evt-1708789200000000000",
  "decision": {
    "allow": true,
    "risk_level": "low",
    "reason": "all_policies_passed",
    "policy_ref": "sha256:abc123...",
    "rule_ids": [],
    "hints": [],
    "reasons": []
  },
  "evidence_record": {
    "event_id": "evt-1708789200000000000",
    "timestamp": "2026-02-25T10:00:00Z",
    "server_id": "evidra-api-01",
    "policy_ref": "sha256:abc123...",
    "actor": {"type": "agent", "id": "deploy-bot", "origin": "api"},
    "tool": "terraform",
    "operation": "apply",
    "environment": "",
    "input_hash": "sha256:def456...",
    "decision": {
      "allow": true,
      "risk_level": "low",
      "reason": "all_policies_passed",
      "rule_ids": [],
      "hints": [],
      "reasons": []
    },
    "signing_payload": "evidra.v1\nevent_id=evt-1708789200000000000\ntimestamp=2026-02-25T10:00:00Z\nserver_id=evidra-api-01\npolicy_ref=sha256:abc123...\ntool=terraform\noperation=apply\nenvironment=\ninput_hash=sha256:def456...\nallow=true\nrisk_level=low\nreason=all_policies_passed\nrule_ids=\nhints=\n",
    "signature": "base64-encoded-ed25519-signature-over-signing_payload"
  }
}
```

The `evidence_record` is a self-contained, server-signed JSON object. The client
persists it wherever they want — local file, S3, ELK, the evidra CLI local
evidence store. The server does not retain it.

The `signature` is computed over the `signing_payload` string — a deterministic
text representation of the record's fields (see §5 for signing details).
Verification is trivial in any language: `ed25519.Verify(pubkey, signing_payload, signature)`.

**Errors:**
- `400` — invalid input (fails `ValidateStructure`)
- `401` — missing or invalid key
- `422` — policy evaluation error
- `429` — rate limit exceeded
- `500` — internal error

Error body:
```json
{
  "ok": false,
  "error": {"code": "invalid_input", "message": "actor.type is required"}
}
```

---

<!-- CHANGED: Removed `GET /v1/evidence/{evidence_id}` endpoint entirely.
     Server has no evidence to return. Replaced with POST /v1/evidence/verify
     and GET /v1/evidence/pubkey. -->

### `POST /v1/evidence/verify`

<!-- CHANGED(v2): No authentication required. This is a public endpoint with
     IP-based rate limiting only. The Ed25519 verify operation uses the public
     key (already exposed at /v1/evidence/pubkey), so gating it behind auth
     adds friction without security benefit. Removed tenant_id check — evidence
     records intentionally do not contain tenant_id. -->

No authentication required (public endpoint, rate-limited by IP).

Accepts an evidence record previously returned by the server and validates
its Ed25519 signature. Allows clients, auditors, and compliance systems to
confirm that a record is authentic and unmodified without needing an API key
or implementing Ed25519 verification themselves.

**Request:**
```json
{
  "evidence_record": {
    "event_id": "evt-1708789200000000000",
    "timestamp": "2026-02-25T10:00:00Z",
    "server_id": "evidra-api-01",
    "policy_ref": "sha256:abc123...",
    "actor": {"type": "agent", "id": "deploy-bot", "origin": "api"},
    "tool": "terraform",
    "operation": "apply",
    "environment": "",
    "input_hash": "sha256:def456...",
    "decision": {
      "allow": true,
      "risk_level": "low",
      "reason": "all_policies_passed",
      "rule_ids": [],
      "hints": [],
      "reasons": []
    },
    "signing_payload": "evidra.v1\nevent_id=evt-1708789200000000000\n...",
    "signature": "base64-encoded-ed25519-signature"
  }
}
```

**Response (200) — valid:**
```json
{
  "ok": true,
  "valid": true
}
```

**Response (200) — invalid:**
```json
{
  "ok": true,
  "valid": false,
  "reason": "signature_mismatch"
}
```

Possible `reason` values: `signature_mismatch`, `malformed_record`, `unknown_key_id`, `payload_field_mismatch`.

The `payload_field_mismatch` reason indicates that the `signing_payload` string
does not match the structured fields in the evidence record. This catches cases
where someone modifies the JSON fields but keeps the original `signing_payload`
(the signature would verify, but the data would be inconsistent). The server
re-derives the signing payload from the structured fields and compares it to the
submitted `signing_payload` before checking the signature.

**Errors:**
- `400` — missing or malformed evidence_record
- `429` — rate limit exceeded

---

### `GET /v1/evidence/pubkey`

No authentication required. Returns the server's Ed25519 public key so clients
can verify evidence records offline without calling the server.

**Response (200):**
```json
{
  "ok": true,
  "key_id": "evidra-api-01",
  "algorithm": "Ed25519",
  "public_key": "base64-encoded-32-byte-ed25519-public-key",
  "created_at": "2026-02-25T00:00:00Z"
}
```

Clients that implement offline verification:
1. Fetch this endpoint once and cache the public key.
2. For each evidence record: verify `ed25519.Verify(pubkey, record.signing_payload, record.signature)`.

No JSON re-serialization needed. The `signing_payload` string is the exact
input that was signed.

---

### `POST /v1/skills`

Register a new skill definition. Requires `Authorization: Bearer <key>`.

**Request:**
```json
{
  "name": "deploy-service",
  "description": "Deploy a service to a Kubernetes cluster",
  "tool": "kubectl",
  "operation": "apply",
  "input_schema": {
    "type": "object",
    "properties": {
      "namespace": { "type": "string" },
      "manifest": { "type": "object" },
      "intent": { "type": "string" }
    },
    "required": ["namespace", "manifest"]
  },
  "risk_tags": ["deployment"],
  "default_environment": "staging",
  "default_target": {
    "cluster": "us-east-1"
  }
}
```

**Response (201):**
```json
{
  "ok": true,
  "skill": {
    "id": "sk_01JEXAMPLE",
    "name": "deploy-service",
    "description": "Deploy a service to a Kubernetes cluster",
    "tool": "kubectl",
    "operation": "apply",
    "input_schema": { "..." : "..." },
    "risk_tags": ["deployment"],
    "default_environment": "staging",
    "default_target": { "cluster": "us-east-1" },
    "created_at": "2026-02-25T10:00:00Z",
    "updated_at": "2026-02-25T10:00:00Z"
  }
}
```

**Errors:**
- `400` — invalid input (missing name, invalid JSON schema, name already exists for tenant)
- `401` — missing or invalid key
- `429` — rate limit

---

### `GET /v1/skills`

List all skills for the authenticated tenant. Requires `Authorization: Bearer <key>`.

**Query parameters:**
- `limit` (int, default 50, max 200)
- `offset` (int, default 0)

**Response (200):**
```json
{
  "ok": true,
  "skills": [
    {
      "id": "sk_01JEXAMPLE",
      "name": "deploy-service",
      "description": "Deploy a service to a Kubernetes cluster",
      "tool": "kubectl",
      "operation": "apply",
      "risk_tags": ["deployment"],
      "default_environment": "staging",
      "created_at": "2026-02-25T10:00:00Z"
    }
  ],
  "total": 1
}
```

---

### `GET /v1/skills/{skill_id}`

Retrieve a single skill definition including the full `input_schema`. Requires `Authorization: Bearer <key>`.

**Response (200):**
```json
{
  "ok": true,
  "skill": {
    "id": "sk_01JEXAMPLE",
    "name": "deploy-service",
    "description": "Deploy a service to a Kubernetes cluster",
    "tool": "kubectl",
    "operation": "apply",
    "input_schema": { "..." : "..." },
    "risk_tags": ["deployment"],
    "default_environment": "staging",
    "default_target": { "cluster": "us-east-1" },
    "created_at": "2026-02-25T10:00:00Z",
    "updated_at": "2026-02-25T10:00:00Z"
  }
}
```

**Errors:**
- `401` — missing or invalid key
- `404` — skill not found (or belongs to another tenant)

---

### `POST /v1/skills/{skill_id}:simulate`

Dry-run policy evaluation. No evidence record is produced. No execution record is created.
Requires `Authorization: Bearer <key>`.

**Request:**
```json
{
  "input": {
    "namespace": "kube-system",
    "manifest": { "kind": "Deployment", "metadata": { "name": "nginx" } }
  },
  "environment": "prod",
  "actor": {
    "type": "agent",
    "id": "deploy-bot",
    "origin": "api"
  }
}
```

**Response (200):**
```json
{
  "ok": true,
  "decision": {
    "allow": false,
    "risk_level": "high",
    "reason": "denied by k8s.protected_namespace",
    "reasons": ["k8s.protected_namespace: namespace kube-system is protected"],
    "rule_ids": ["k8s.protected_namespace"],
    "hints": ["Use a non-system namespace for deployments"]
  }
}
```

**Errors:**
- `400` — input fails skill schema validation
- `401` — missing or invalid key
- `404` — skill not found
- `422` — policy evaluation error

---

### `POST /v1/skills/{skill_id}:execute`

<!-- CHANGED: Response now includes `evidence_record` with server signature.
     No evidence written to Postgres. Lightweight execution record stored
     with 90-day retention. -->

Execute policy evaluation, create lightweight execution record, return signed evidence.
Requires `Authorization: Bearer <key>`.

**Request:**
```json
{
  "input": {
    "namespace": "production",
    "manifest": { "kind": "Deployment", "metadata": { "name": "api-v2" } },
    "intent": "deploy api v2.3"
  },
  "environment": "prod",
  "actor": {
    "type": "agent",
    "id": "deploy-bot",
    "origin": "api"
  },
  "idempotency_key": "deploy-api-v2.3-20260225T100000"
}
```

<!-- CHANGED(v2): evidence_record includes signing_payload field. -->

**Response (200) — allowed:**
```json
{
  "ok": true,
  "execution_id": "ex_01JEXAMPLE",
  "event_id": "evt-1740477600000000000",
  "decision": {
    "allow": true,
    "risk_level": "low",
    "reason": "all_policies_passed",
    "rule_ids": [],
    "hints": []
  },
  "status": "allowed",
  "skill_id": "sk_01JEXAMPLE",
  "created_at": "2026-02-25T10:00:00Z",
  "evidence_record": {
    "event_id": "evt-1740477600000000000",
    "timestamp": "2026-02-25T10:00:00Z",
    "server_id": "evidra-api-01",
    "policy_ref": "sha256:abc123...",
    "skill_id": "sk_01JEXAMPLE",
    "execution_id": "ex_01JEXAMPLE",
    "actor": {"type": "agent", "id": "deploy-bot", "origin": "api"},
    "tool": "kubectl",
    "operation": "apply",
    "environment": "prod",
    "input_hash": "sha256:def456...",
    "decision": {
      "allow": true,
      "risk_level": "low",
      "reason": "all_policies_passed",
      "rule_ids": [],
      "hints": []
    },
    "signing_payload": "evidra.v1\nevent_id=evt-1740477600000000000\n...",
    "signature": "base64-encoded-ed25519-signature-over-signing_payload"
  }
}
```

**Response (200) — denied:**
```json
{
  "ok": true,
  "execution_id": "ex_01JEXAMPLE2",
  "event_id": "evt-1740477600000000001",
  "decision": {
    "allow": false,
    "risk_level": "high",
    "reason": "denied by k8s.protected_namespace",
    "reasons": ["k8s.protected_namespace: namespace kube-system is protected"],
    "rule_ids": ["k8s.protected_namespace"],
    "hints": ["Use a non-system namespace for deployments"]
  },
  "status": "denied",
  "skill_id": "sk_01JEXAMPLE",
  "created_at": "2026-02-25T10:00:00Z",
  "evidence_record": {
    "event_id": "evt-1740477600000000001",
    "timestamp": "2026-02-25T10:00:00Z",
    "server_id": "evidra-api-01",
    "policy_ref": "sha256:abc123...",
    "skill_id": "sk_01JEXAMPLE",
    "execution_id": "ex_01JEXAMPLE2",
    "actor": {"type": "agent", "id": "deploy-bot", "origin": "api"},
    "tool": "kubectl",
    "operation": "apply",
    "environment": "prod",
    "input_hash": "sha256:ghi789...",
    "decision": {
      "allow": false,
      "risk_level": "high",
      "reason": "denied by k8s.protected_namespace",
      "rule_ids": ["k8s.protected_namespace"],
      "hints": ["Use a non-system namespace for deployments"]
    },
    "signing_payload": "evidra.v1\nevent_id=evt-1740477600000000001\n...",
    "signature": "base64-encoded-ed25519-signature-over-signing_payload"
  }
}
```

Note: a denied execution returns `200` with `"ok": true` and `"status": "denied"`. The HTTP status reflects that the request was processed correctly. The `decision.allow` field carries the policy verdict.

<!-- CHANGED(v2): Idempotent replay no longer returns a cached evidence_record.
     The execution row stores only metadata. The client is responsible for
     caching the evidence_record from the first response. -->

**Idempotency:** If `idempotency_key` matches an existing execution for this
tenant, a minimal replay response is returned without re-evaluation:

```json
{
  "ok": true,
  "idempotent_replay": true,
  "execution_id": "ex_01JEXAMPLE",
  "event_id": "evt-1740477600000000000",
  "decision": {
    "allow": true,
    "risk_level": "low"
  },
  "status": "allowed",
  "message": "This request was already processed. Use your previously received evidence_record for audit purposes."
}
```

The replay response is built from the execution row's stored metadata
(`decision_allow`, `decision_risk`, `event_id`, `status`). No full evidence
record is returned — the client must cache the `evidence_record` from the
original response. Idempotency keys expire with execution row cleanup (90 days).

**Errors:**
- `400` — input fails skill schema validation, or missing required fields
- `401` — missing or invalid key
- `404` — skill not found
- `422` — policy evaluation error
- `429` — rate limit
- `500` — internal error

---

### `GET /v1/executions/{execution_id}`

<!-- CHANGED: Returns lightweight execution record. No full evidence payload.
     `event_id` is a plain string field, not a FK to evidence table. -->

Retrieve a lightweight execution record. Requires `Authorization: Bearer <key>`.

**Response (200):**
```json
{
  "ok": true,
  "execution": {
    "id": "ex_01JEXAMPLE",
    "skill_id": "sk_01JEXAMPLE",
    "skill_name": "deploy-service",
    "status": "allowed",
    "decision_allow": true,
    "decision_risk": "low",
    "input_hash": "sha256:abc123...",
    "event_id": "evt-1740477600000000000",
    "environment": "prod",
    "actor": {
      "type": "agent",
      "id": "deploy-bot",
      "origin": "api"
    },
    "idempotency_key": "deploy-api-v2.3-20260225T100000",
    "created_at": "2026-02-25T10:00:00Z"
  }
}
```

<!-- CHANGED(v2): Removed mention of evidence_record_jsonb cache. The server
     stores no evidence payload. -->

The `event_id` is a reference the client can use to correlate with their
locally-stored evidence record. The server does not store evidence payloads.

**Errors:**
- `401` — missing or invalid key
- `404` — execution not found (or belongs to another tenant)

---

### `GET /healthz`

No auth. Returns `200 OK` with `{"status": "ok"}`. Checks that the process is
running and the OPA engine is loaded.

### `GET /readyz`

No auth. Returns `200 OK` only when Postgres is reachable (a `SELECT 1` probe).
Returns `503` otherwise. Used by orchestrators for readiness gating.

---

### Rate Limits

**Per-key limits:**

| Endpoint | Limit |
|---|---|
| `POST /v1/validate` | 60 req/min |
| `POST /v1/skills` | 30 req/min |
| `GET /v1/skills`, `GET /v1/skills/{id}` | 120 req/min |
| `POST /v1/skills/{id}:simulate` | 60 req/min |
| `POST /v1/skills/{id}:execute` | 60 req/min |
| `GET /v1/executions/{id}` | 120 req/min |

Burst allowance: 2x the per-minute rate.

<!-- CHANGED(v2): Moved POST /v1/evidence/verify from per-key to per-IP limits.
     It is now a public endpoint. -->

**Per-IP limits (unauthenticated):**
- `POST /v1/keys`: 5 requests/hour per IP
- `POST /v1/evidence/verify`: 60 requests/minute per IP
- `GET /v1/evidence/pubkey`: 60 requests/minute per IP
- Global: 100 requests/minute per IP across all endpoints

**Implementation:** In-memory token bucket (`golang.org/x/time/rate`) keyed by
tenant_id (authenticated) or IP (unauthenticated). Sufficient for single-node
MVP. Returns `429 Too Many Requests` with `Retry-After` header.

---

## 3. Data Model (PostgreSQL)

<!-- CHANGED: Removed `evidence` table entirely. Removed `execution_evidence`
     join table. `executions` table stores `event_id` as a plain TEXT field
     (not a FK). -->

<!-- CHANGED(v2): Removed `evidence_record_jsonb` from `executions` table.
     No evidence payload stored server-side at all. -->

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

### `skills`

```sql
CREATE TABLE skills (
    id                  TEXT        PRIMARY KEY,              -- "sk_" + ULID
    tenant_id           TEXT        NOT NULL REFERENCES tenants(id),
    name                TEXT        NOT NULL,
    description         TEXT        NOT NULL DEFAULT '',
    tool                TEXT        NOT NULL,                  -- maps to ToolInvocation.Tool
    operation           TEXT        NOT NULL,                  -- maps to ToolInvocation.Operation
    input_schema        JSONB       NOT NULL DEFAULT '{}',     -- JSON Schema draft-07
    risk_tags           TEXT[]      NOT NULL DEFAULT '{}',
    default_environment TEXT        NOT NULL DEFAULT '',
    default_target      JSONB,                                 -- merged into params.target
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ                            -- soft delete; NULL = active
);

CREATE UNIQUE INDEX idx_skills_tenant_name ON skills (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_skills_tenant ON skills (tenant_id) WHERE deleted_at IS NULL;
```

**Constraints:**
- `name` is unique per tenant (among non-deleted skills).
- `name` must match `^[a-z][a-z0-9_-]{1,62}[a-z0-9]$` (3–64 chars, lowercase, hyphens/underscores allowed, no leading/trailing special chars).
- `input_schema` is validated at registration time. Invalid JSON Schema is rejected with `400`.
- Maximum `input_schema` size: 64 KB.
- Maximum 200 skills per tenant in MVP.

---

### `executions`

<!-- CHANGED: No FK to evidence table. `event_id` is a plain TEXT field storing
     the reference. Removed `execution_evidence` join table. -->

<!-- CHANGED(v2): Removed `evidence_record_jsonb` column. Execution rows store
     only metadata (~100-150 bytes/row). Idempotent replay returns a minimal
     response built from the stored metadata columns. -->

```sql
CREATE TABLE executions (
    id                TEXT        PRIMARY KEY,          -- "ex_" + ULID
    tenant_id         TEXT        NOT NULL REFERENCES tenants(id),
    skill_id          TEXT        NOT NULL REFERENCES skills(id),
    status            TEXT        NOT NULL,              -- "allowed" | "denied" | "error"
    decision_allow    BOOLEAN     NOT NULL,
    decision_risk     TEXT        NOT NULL,              -- "low" | "medium" | "high"
    decision_reason   TEXT        NOT NULL DEFAULT '',
    rule_ids          TEXT[]      NOT NULL DEFAULT '{}',
    event_id          TEXT        NOT NULL,              -- reference only, not a FK
    environment       TEXT        NOT NULL DEFAULT '',
    actor_type        TEXT        NOT NULL,
    actor_id          TEXT        NOT NULL,
    actor_origin      TEXT        NOT NULL,
    input_hash        TEXT        NOT NULL,              -- SHA-256 of validated input
    idempotency_key   TEXT,                              -- optional, unique per tenant
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_executions_tenant_created ON executions (tenant_id, created_at DESC);
CREATE INDEX idx_executions_tenant_skill ON executions (tenant_id, skill_id, created_at DESC);
CREATE UNIQUE INDEX idx_executions_idempotency
    ON executions (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

**Notes:**
- `status` is terminal at creation time. No status transitions in MVP (no async execution).
- `event_id` is a plain reference string (e.g. `"evt-1740477600000000000"`). It allows correlation with client-side evidence stores but is not a foreign key — the server does not store evidence.
- `idempotency_key` uniqueness is enforced per tenant. A daily cleanup job deletes execution rows older than 90 days, which also removes stale idempotency keys.
- Idempotent replay builds the response from the stored metadata columns (`decision_allow`, `decision_risk`, `event_id`, `status`). No evidence payload is cached or returned on replay.

---

### `usage_counters`

Pre-aggregated counters in 1-hour buckets. No raw request logs for MVP — this
keeps storage bounded and avoids PII concerns.

```sql
CREATE TABLE usage_counters (
    tenant_id       TEXT        NOT NULL REFERENCES tenants(id),
    endpoint        TEXT        NOT NULL,     -- "validate" | "execute" | "simulate" | "skills" | "keys" | "verify"
    bucket          TIMESTAMPTZ NOT NULL,     -- truncated to hour
    request_count   BIGINT      NOT NULL DEFAULT 0,
    allow_count     BIGINT      NOT NULL DEFAULT 0,   -- only for "validate" / "execute"
    deny_count      BIGINT      NOT NULL DEFAULT 0,   -- only for "validate" / "execute"
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

---

### Retention

<!-- CHANGED: Removed `evidence` and `execution_evidence` rows from retention table.
     All server-side data has bounded retention. -->

| Table | Retention | Cleanup |
|---|---|---|
| `tenants` | Indefinite | — |
| `api_keys` | Indefinite (revoked keys kept for audit) | — |
| `skills` | Indefinite (soft-deleted rows cleaned after 90 days) | Daily `DELETE WHERE deleted_at < now() - '90 days'` |
| `executions` | 90 days | Daily `DELETE WHERE created_at < now() - '90 days'` |
| `usage_counters` | 90 days | Daily `DELETE WHERE bucket < now() - '90 days'` |

---

### Migrations

<!-- CHANGED: Single migration file. No evidence table. No execution_evidence table. -->

```
internal/migrate/migrations/
  001_initial.up.sql      -- tenants, api_keys, usage_counters, skills, executions
  001_initial.down.sql
```

---

## 4. Skill Definition Schema

A skill maps a named action to the existing `ToolInvocation` model. The skill definition controls how the `ToolInvocation` is built from the caller's input.

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | auto | `"sk_" + ULID`, assigned at creation |
| `name` | string | yes | Unique per tenant. Lowercase, 3–64 chars. |
| `description` | string | no | Human-readable description. Max 1024 chars. |
| `tool` | string | yes | Maps to `ToolInvocation.Tool`. E.g. `"kubectl"`, `"terraform"`, `"aws"`. |
| `operation` | string | yes | Maps to `ToolInvocation.Operation`. E.g. `"apply"`, `"delete"`, `"plan"`. |
| `input_schema` | object | no | JSON Schema (draft-07) for validating caller input. Defaults to `{}` (any object). |
| `risk_tags` | string[] | no | Attached to every `ToolInvocation.Params.risk_tags`. Set by the skill registrar. |
| `default_environment` | string | no | Used when caller does not specify `environment`. |
| `default_target` | object | no | Merged (caller overrides) into `ToolInvocation.Params.target`. |

### ToolInvocation Mapping

When a skill is executed, the backend constructs a `ToolInvocation` as follows:

```
ToolInvocation {
  Actor:       request.actor                       // required in execute request
  Tool:        skill.tool                           // from skill definition
  Operation:   skill.operation                      // from skill definition
  Params: {
    target:    merge(skill.default_target, input)   // skill defaults + caller overrides
    payload:   input (after schema validation)       // caller's validated input
    risk_tags: skill.risk_tags                       // from skill definition (immutable)
  }
  Context: {
    source:    "skills-api"                          // fixed
    intent:    input.intent (if provided)            // optional caller field
  }
  Environment: input.environment || skill.default_environment
}
```

### Risk Tags

Risk tags are set at skill registration time by the skill registrar (the API key holder). They cannot be overridden by the caller at execution time. This prevents privilege escalation — an agent cannot add `"breakglass"` or `"change-approved"` tags to bypass policy.

### Example: Full Skill Definition

```json
{
  "id": "sk_01JEXAMPLE",
  "name": "scale-deployment",
  "description": "Scale a Kubernetes deployment replica count",
  "tool": "kubectl",
  "operation": "scale",
  "input_schema": {
    "type": "object",
    "properties": {
      "namespace": {
        "type": "string",
        "minLength": 1,
        "maxLength": 63
      },
      "deployment": {
        "type": "string",
        "minLength": 1
      },
      "replicas": {
        "type": "integer",
        "minimum": 0,
        "maximum": 100
      },
      "intent": {
        "type": "string",
        "maxLength": 256
      }
    },
    "required": ["namespace", "deployment", "replicas"],
    "additionalProperties": false
  },
  "risk_tags": ["scaling"],
  "default_environment": "staging",
  "default_target": {
    "cluster": "us-east-1"
  },
  "created_at": "2026-02-25T10:00:00Z",
  "updated_at": "2026-02-25T10:00:00Z"
}
```

---

## 5. Security Design

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

### API Key Hashing

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

<!-- CHANGED: Added evidence record signing section (Ed25519). This is the core
     change — replaces server-side evidence storage with cryptographic signing. -->

### Evidence Record Signing (Ed25519)

**Choice: Ed25519 over HMAC-SHA256.**

HMAC-SHA256 is simpler but requires sharing the secret key for verification.
Since evidence records live on the client side and may need to be verified by
third parties (auditors, compliance systems), HMAC would require either:
(a) sharing the HMAC secret (defeats the purpose), or (b) always calling the
server to verify (creates a dependency). Ed25519 solves both: the server signs
with a private key; anyone with the public key can verify offline.

Additional advantages of Ed25519:
- Non-repudiation: the server demonstrably signed the record.
- Small signatures: 64 bytes.
- Fast: ~70,000 signatures/sec on modern hardware. No performance concern.
- Standard library support: Go `crypto/ed25519`.

**Key management:**

```go
// At startup:
// 1. Load Ed25519 private key from EVIDRA_SIGNING_KEY_PATH (PEM file)
//    or EVIDRA_SIGNING_KEY (base64-encoded 64-byte seed+key in env var)
// 2. If neither is set, generate a new keypair and write to
//    data/signing_key.pem (first-run only; logged as warning)
// 3. Derive public key from private key
// 4. Set server_id (key_id) from EVIDRA_SERVER_ID env var or hostname

type Signer struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
    keyID      string  // e.g. "evidra-api-01"
}
```

<!-- CHANGED(v2): Replaced json.Marshal canonical serialization with explicit
     signing_payload — a deterministic text format that any language can
     reconstruct. This eliminates cross-language JSON serialization mismatches
     (field ordering, omitempty, null vs [], whitespace). -->

**Signing payload format:**

The signature is computed over a deterministic text string, not JSON. The
`signing_payload` is constructed by concatenating fields in a fixed order with
`key=value\n` delimiters:

```
evidra.v1\n
event_id={event_id}\n
timestamp={RFC3339 UTC}\n
server_id={server_id}\n
policy_ref={policy_ref}\n
skill_id={skill_id or empty}\n
execution_id={execution_id or empty}\n
actor_type={actor.type}\n
actor_id={actor.id}\n
actor_origin={actor.origin}\n
tool={tool}\n
operation={operation}\n
environment={environment or empty}\n
input_hash={input_hash}\n
allow={true|false}\n
risk_level={risk_level}\n
reason={reason}\n
rule_ids={comma-separated, sorted alphabetically}\n
hints={comma-separated, sorted alphabetically}\n
```

Rules:
- Version prefix `evidra.v1\n` is always the first line.
- All fields are present in every payload (empty string for absent values).
- Lists (`rule_ids`, `hints`) are sorted alphabetically and joined with `,` (no spaces).
- Empty lists produce an empty value after `=`.
- Timestamps are always RFC3339 in UTC with `Z` suffix (no timezone offset).
- No trailing newline after the last field.

This format is trivially constructible in any language — no JSON serializer
ambiguity, no struct field ordering dependency, no `null` vs `[]` vs `""` issues.

**Signing process:**

```go
func (s *Signer) BuildSigningPayload(rec *EvidenceRecord) string {
    var b strings.Builder
    b.WriteString("evidra.v1\n")
    fmt.Fprintf(&b, "event_id=%s\n", rec.EventID)
    fmt.Fprintf(&b, "timestamp=%s\n", rec.Timestamp.UTC().Format(time.RFC3339))
    fmt.Fprintf(&b, "server_id=%s\n", rec.ServerID)
    fmt.Fprintf(&b, "policy_ref=%s\n", rec.PolicyRef)
    fmt.Fprintf(&b, "skill_id=%s\n", rec.SkillID)
    fmt.Fprintf(&b, "execution_id=%s\n", rec.ExecutionID)
    fmt.Fprintf(&b, "actor_type=%s\n", rec.Actor.Type)
    fmt.Fprintf(&b, "actor_id=%s\n", rec.Actor.ID)
    fmt.Fprintf(&b, "actor_origin=%s\n", rec.Actor.Origin)
    fmt.Fprintf(&b, "tool=%s\n", rec.Tool)
    fmt.Fprintf(&b, "operation=%s\n", rec.Operation)
    fmt.Fprintf(&b, "environment=%s\n", rec.Environment)
    fmt.Fprintf(&b, "input_hash=%s\n", rec.InputHash)
    fmt.Fprintf(&b, "allow=%t\n", rec.Decision.Allow)
    fmt.Fprintf(&b, "risk_level=%s\n", rec.Decision.RiskLevel)
    fmt.Fprintf(&b, "reason=%s\n", rec.Decision.Reason)
    fmt.Fprintf(&b, "rule_ids=%s\n", sortedJoin(rec.Decision.RuleIDs))
    fmt.Fprintf(&b, "hints=%s", sortedJoin(rec.Decision.Hints))
    return b.String()
}

func (s *Signer) Sign(rec *EvidenceRecord) error {
    rec.ServerID = s.keyID
    payload := s.BuildSigningPayload(rec)
    rec.SigningPayload = payload
    sig := ed25519.Sign(s.privateKey, []byte(payload))
    rec.Signature = base64.StdEncoding.EncodeToString(sig)
    return nil
}

func (s *Signer) Verify(rec *EvidenceRecord) (bool, string) {
    // 1. Verify signature over signing_payload
    sig, err := base64.StdEncoding.DecodeString(rec.Signature)
    if err != nil || len(sig) != ed25519.SignatureSize {
        return false, "malformed_record"
    }
    if !ed25519.Verify(s.publicKey, []byte(rec.SigningPayload), sig) {
        return false, "signature_mismatch"
    }
    // 2. Verify signing_payload matches the structured fields
    expected := s.BuildSigningPayload(rec)
    if rec.SigningPayload != expected {
        return false, "payload_field_mismatch"
    }
    return true, ""
}

func sortedJoin(ss []string) string {
    if len(ss) == 0 {
        return ""
    }
    sorted := make([]string, len(ss))
    copy(sorted, ss)
    sort.Strings(sorted)
    return strings.Join(sorted, ",")
}
```

**Key rotation:**

MVP uses a single key. For rotation (post-MVP):
1. Generate new keypair.
2. Publish both old and new public keys at `/v1/evidence/pubkey` (returns array).
3. New signatures use the new key. Old signatures remain verifiable with the old public key.
4. Each evidence record's `server_id` identifies which key signed it.

### Evidence Record Schema

<!-- CHANGED: New section — defines the evidence record that is returned to clients. -->

<!-- CHANGED(v2): Added `SigningPayload` field. Removed `omitempty` from
     SkillID, ExecutionID, Environment — all fields are always present in the
     signing payload, so they must always be present in the struct. -->

The `evidence_record` is a self-contained JSON object:

```go
type EvidenceRecord struct {
    EventID        string          `json:"event_id"`         // "evt-<UnixNano>"
    Timestamp      time.Time       `json:"timestamp"`        // UTC
    ServerID       string          `json:"server_id"`        // key_id of signing key
    PolicyRef      string          `json:"policy_ref"`       // SHA-256 of policy bundle
    SkillID        string          `json:"skill_id"`         // set for execute, "" for validate
    ExecutionID    string          `json:"execution_id"`     // set for execute, "" for validate
    Actor          invocation.Actor `json:"actor"`
    Tool           string          `json:"tool"`
    Operation      string          `json:"operation"`
    Environment    string          `json:"environment"`
    InputHash      string          `json:"input_hash"`       // SHA-256 of canonical input JSON
    Decision       DecisionRecord  `json:"decision"`
    SigningPayload string          `json:"signing_payload"`  // deterministic text, input to Ed25519
    Signature      string          `json:"signature"`        // base64(Ed25519(signing_payload))
}

type DecisionRecord struct {
    Allow     bool     `json:"allow"`
    RiskLevel string   `json:"risk_level"`
    Reason    string   `json:"reason"`
    RuleIDs   []string `json:"rule_ids"`
    Hints     []string `json:"hints"`
    Reasons   []string `json:"reasons"`
}
```

**What is NOT in the evidence record:**
- No plaintext input payload (only `input_hash`). This prevents secrets
  leakage if the evidence record is stored in a less-secure location.
- No tenant_id (the API key holder knows their tenant; embedding it would
  leak tenant structure if records are shared).
- No chain hashes (hash-linked chains are a client-side concern — the local
  evidra evidence store implements chaining).

### Threat Model

| Threat | Mitigation |
|---|---|
| **Evidence forgery** | Ed25519 signature over deterministic `signing_payload`. Clients verify with the server's public key. Forging requires the private key. |
| **Evidence tampering** | Signature covers the `signing_payload` which encodes all decision-relevant fields. `POST /v1/evidence/verify` also checks that structured JSON fields match the `signing_payload` (detects payload/field desync). |
| **Replay of evidence records** | Each record has a unique `event_id` (nanosecond timestamp). Clients deduplicate by `event_id`. |
| **Input injection via skill input** | Input validated against registered `input_schema` before `ToolInvocation` construction. Schema is set at registration time, not by the caller. |
| **Risk tag escalation** | `risk_tags` are fixed in the skill definition. Execute request cannot override them. |
| **Replay of API requests** | `idempotency_key` deduplication. TLS required for all external traffic. |
| **Privilege escalation via skill mutation** | Skills can only be mutated by the same tenant's API key. No cross-tenant access. |
| **SSRF** | The server does not make outbound HTTP calls. It only evaluates policy locally. |
| **Signing key compromise** | Key is loaded from a file or env var, never logged, never exposed via API. Rotation procedure documented. |
| **Enumeration** | 404 returned for all not-found cases regardless of existence. IDs use ULIDs (not sequential). |
| **Verify endpoint abuse** | `POST /v1/evidence/verify` is public but IP rate-limited (60 req/min). No secrets are exposed — it performs a mathematical operation using the public key. |

### Abuse Mitigations

- **IP throttling** on key issuance prevents key farming.
- **Key revocation:** `api_keys.revoked_at` — set via internal admin query for
  MVP (no admin API yet). Auth middleware checks `revoked_at IS NULL`.
- **Request size limit:** 1 MB max body on all endpoints. Enforced by
  `http.MaxBytesReader`.
- **Idle key cleanup:** Keys not used in 90 days are candidates for revocation
  (manual review for MVP; automated later).
- Max 200 skills per tenant.
- Max `input_schema` size: 64 KB.
- Skill name regex enforced at registration.

### Tenant Isolation

Every storage query includes `WHERE tenant_id = $1`. The `tenant_id` is
extracted from the authenticated API key in middleware and injected into
`context.Context`. Handlers never accept tenant_id from request parameters.

Execution retrieval returns 404 (not 403) for records belonging to other
tenants, preventing existence enumeration.

### Logging / Redaction Rules

- **Never log:** plaintext API keys, request body `params` or `context` contents
  (may contain infrastructure details), Ed25519 private key material.
- **Always log:** key prefix, tenant_id, endpoint, status code, latency,
  event_id, execution_id (if applicable), skill_id (if applicable),
  decision (allow/deny/risk_level).
- **Structured JSON logging** via `log/slog` with redaction enforced at the
  logger level — sensitive fields are not passed to the logger in the first
  place (not filtered after the fact).

---

## 6. Execution Semantics

### Simulate vs Validate vs Execute

| | Simulate | Validate | Execute |
|---|---|---|---|
| Endpoint | `POST /v1/skills/{id}:simulate` | `POST /v1/validate` | `POST /v1/skills/{id}:execute` |
| Input | Skill input (schema-validated) | Raw `ToolInvocation` | Skill input (schema-validated) |
| Policy evaluation | Yes | Yes | Yes |
| Evidence record returned | **No** | **Yes** (signed) | **Yes** (signed) |
| Execution record in DB | **No** | **No** | **Yes** (lightweight, 90 days) |
| Usage counter | Incremented (`simulate`) | Incremented (`validate`) | Incremented (`execute`) |
| Idempotency key | Not supported | Not supported | Supported |

`simulate` is for dry-run / pre-flight checks. Agents can call simulate to
preview a decision before committing. No evidence is produced — it is purely
advisory.

`validate` is the raw policy evaluation endpoint. It accepts a `ToolInvocation`
directly (no skill definition needed) and returns a signed evidence record.

`execute` is the skills-based flow. It loads a skill definition, validates
input against the schema, evaluates policy, creates an execution record, and
returns a signed evidence record.

### When Evidence Records Are Produced

<!-- CHANGED: Evidence is never written to the DB. It is always returned in the
     response body as a signed JSON object. -->

Evidence records are produced by `validate` and `execute` endpoints. They are
**returned in the response body**, not stored on the server. The client owns
the evidence record from the moment it receives the response.

Evidence records are produced **regardless** of whether the decision is allow
or deny. A denied execution still produces a signed evidence record — the
client can use it to prove that the policy evaluation happened and what the
outcome was.

### Decision and Execution State

| Decision | Execution Status | Meaning |
|---|---|---|
| `allow: true` | `"allowed"` | Policy passed. Platform should proceed with the infrastructure action. |
| `allow: false` | `"denied"` | Policy blocked. Platform should not proceed. |
| (evaluation error) | `"error"` | Policy engine failed. Platform should treat as deny. |

Execution status is terminal at creation. There are no status transitions in MVP.

### Idempotency

<!-- CHANGED(v2): Idempotent replay returns a minimal response from execution
     metadata, not a cached evidence_record. Client must cache evidence from
     the original response. -->

- The `idempotency_key` field is optional on execute requests.
- If provided, the backend checks for an existing execution with the same
  `(tenant_id, idempotency_key)` in the `executions` table.
- If found, a **minimal replay response** is returned without re-evaluation.
  The replay response includes `execution_id`, `event_id`, `status`, and the
  decision summary (`allow`, `risk_level`) from the stored metadata columns.
  It does **not** include a full `evidence_record` — the client must have
  cached the evidence record from the original response.
- If not found, a new execution proceeds normally.
- Keys are strings, max 128 chars, set by the caller.
  Recommended format: `{skill-name}-{unique-context}-{timestamp}`.
- Idempotency keys are cleaned up with execution rows at 90-day retention.

### Timeouts

- Policy evaluation timeout: 5 seconds (OPA engine). If exceeded, returns `422` with `"code": "policy_timeout"`.
- Request timeout: 30 seconds (HTTP server). Covers full request lifecycle.
- Database query timeout: 10 seconds.

### Retries

The server does not retry internally. If a transient failure occurs (DB
unavailable, lock contention), the endpoint returns an error and the caller
retries. Idempotency keys make retries safe for execute.

---

## 7. Reuse of Existing Engine

### Entry point

The API calls the OPA engine through an adapter that initializes a
`runtime.Evaluator` once at startup:

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

### Profile selection

MVP bundles the single `ops-v0.1` profile. The `PolicyPath` and `DataPath` are
resolved at startup (either embedded in the binary or read from a config path).
The `?profile=` query parameter is accepted but ignored for now — it exists so
clients can start passing it without breaking when multi-profile is added.

<!-- CHANGED: Removed "Evidence linking to tenant" section. Evidence is no longer
     written to Postgres. Instead, the handler builds an evidence record in
     memory and signs it. -->

<!-- CHANGED(v2): Builder now constructs signing_payload and calls signer with
     the payload-based approach. -->

### Building and signing evidence

After `Evaluate` returns, the handler builds an evidence record in memory:

```go
rec := evidence.EvidenceRecord{
    EventID:     fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano()),
    Timestamp:   time.Now().UTC(),
    PolicyRef:   decision.PolicyRef,
    Actor:       inv.Actor,
    Tool:        inv.Tool,
    Operation:   inv.Operation,
    Environment: inv.Environment,
    InputHash:   sha256OfInput(inv),
    Decision: evidence.DecisionRecord{
        Allow:     decision.Allow,
        RiskLevel: decision.RiskLevel,
        Reason:    decision.Reason,
        RuleIDs:   decision.Hits,
        Hints:     decision.Hints,
        Reasons:   decision.Reasons,
    },
}
// For execute: also set SkillID and ExecutionID
// For validate: SkillID and ExecutionID remain ""
signer.Sign(&rec) // builds signing_payload, signs it, sets both fields
// Return rec in response body — no DB write for evidence
```

---

## 8. Code Architecture (Go)

<!-- CHANGED: Removed internal/storage/evidence.go and
     internal/storage/execution_evidence.go. Removed internal/api/evidence_handler.go.
     Added internal/evidence/ package (signer, builder, types).
     Added internal/api/verify_handler.go. -->

```
cmd/
  evidra-api/
    main.go                  # Entrypoint: parse env, init DB, init engine,
                             # init signer, start server

internal/
  api/
    router.go                # stdlib http.ServeMux, mounts handlers
    middleware.go            # Request logging, recovery, request-id, CORS, body limit
    keys_handler.go          # POST /v1/keys
    validate_handler.go      # POST /v1/validate
    skills_handler.go        # POST/GET /v1/skills, GET /v1/skills/{id}
    execute_handler.go       # POST /v1/skills/{id}:simulate, POST /v1/skills/{id}:execute,
                             # GET /v1/executions/{id}
    verify_handler.go        # POST /v1/evidence/verify (public), GET /v1/evidence/pubkey
    health_handler.go        # GET /healthz, /readyz
    response.go              # JSON response helpers, error formatting

  auth/
    middleware.go            # Bearer token extraction, key lookup, tenant context injection
    apikey.go                # Key generation, parsing, hashing functions
    context.go               # TenantID get/set on context.Context

  engine/
    adapter.go               # Thin wrapper: calls pkg/runtime.Evaluator directly
                             # Manages OPA engine lifecycle (init once at startup)

  evidence/
    signer.go                # Ed25519 signing and verification, signing payload builder
    builder.go               # Build EvidenceRecord from Decision + ToolInvocation
    types.go                 # EvidenceRecord, DecisionRecord structs

  skills/
    validator.go             # JSON Schema validation (registration + input)
    builder.go               # BuildInvocation(skill, input, actor, env) → ToolInvocation

  storage/
    postgres.go              # *sql.DB initialization, connection pool, ping
    tenants.go               # TenantRepo: Create
    apikeys.go               # APIKeyRepo: Create, FindByHash, Revoke, TouchLastUsed
    skills.go                # SkillRepo: Create, FindByID, FindByName, ListByTenant,
                             #   Update, SoftDelete
    executions.go            # ExecutionRepo: Create, FindByID, FindByIdempotencyKey
    usage.go                 # UsageRepo: Increment (upsert usage_counters)

  ratelimit/
    limiter.go               # Token bucket per key/IP, cleanup goroutine

  migrate/
    migrations/
      001_initial.up.sql     # CREATE TABLE tenants, api_keys, usage_counters,
                             #   skills, executions
      001_initial.down.sql
```

### Dependency Direction

```
cmd/evidra-api
  → internal/api        (HTTP layer)
  → internal/auth       (middleware)
  → internal/engine     (adapter)
  → internal/evidence   (signer)
  → internal/storage    (repos)
  → internal/ratelimit

internal/api
  → internal/auth       (extracts tenant from context)
  → internal/engine     (calls Evaluate)
  → internal/evidence   (signs and verifies evidence records)
  → internal/skills     (validates input, builds ToolInvocation)
  → internal/storage    (reads/writes skills, executions, keys, usage)
  → internal/ratelimit

internal/auth
  → internal/storage    (looks up key hash)

internal/engine
  → pkg/validate        (calls EvaluateInvocation)
  → pkg/invocation      (ToolInvocation type)

internal/evidence
  → pkg/invocation      (Actor type for EvidenceRecord)
  → (crypto/ed25519, no other internal/ imports)

internal/skills
  → pkg/invocation      (builds ToolInvocation)
  → (no other internal/ imports)

internal/storage
  → (database/sql only; no pkg/ imports)

pkg/*
  → (no internal/ imports — engine is unaware of HTTP or Postgres)
```

**Key boundary:** `internal/engine` and `internal/evidence` are the only
packages that import `pkg/*`. Both are thin adapters — no business logic.

---

## 9. Observability

### Structured Log Fields

Every request logs these fields via `slog`:

```json
{
  "level": "info",
  "msg": "request_completed",
  "request_id": "req-abc123",
  "tenant_id": "01JEXAMPLE",
  "key_prefix": "ev1_a8Fk3mQ",
  "method": "POST",
  "path": "/v1/skills/sk_01JEXAMPLE:execute",
  "http_status": 200,
  "duration_ms": 12,
  "event_id": "evt-1740477600000000000",
  "execution_id": "ex_01JEXAMPLE",
  "skill_id": "sk_01JEXAMPLE",
  "decision_allow": true,
  "decision_risk": "low",
  "status": "allowed"
}
```

Log levels:
- `info` — request completed (no payload content logged).
- `warn` — rate limit hit, idempotency key collision, input validation failure.
- `error` — policy engine error, DB error.

### Prometheus Metrics

```
evidra_api_requests_total{endpoint, status_code}         counter
evidra_api_request_duration_seconds{endpoint}             histogram
evidra_api_decisions_total{allow, risk_level}              counter
evidra_api_keys_issued_total                               counter
evidra_api_active_connections                               gauge
evidra_skills_total{tenant_id}                             gauge
evidra_skill_executions_total{skill_id, status, risk_level}  counter
evidra_skill_execution_duration_seconds{skill_id}           histogram
evidra_skill_simulations_total{skill_id, allow}             counter
evidra_skill_input_validation_failures_total{skill_id}      counter
evidra_evidence_signatures_total                            counter
evidra_evidence_verifications_total{valid}                   counter
```

Exposed at `GET /metrics` (behind a separate port or basic auth for MVP).

### Audit Logging

<!-- CHANGED: No server-side evidence table. The signed evidence record in the
     response IS the audit trail, owned by the client. -->

- `validate` and `execute` calls return signed evidence records. This is the
  primary audit mechanism — the client persists the records.
- `simulate` calls are counted via `usage_counters` but produce no evidence.
- Server-side `usage_counters` provide aggregate analytics (request counts,
  allow/deny ratios, latency) without storing per-request audit data.

---

## 10. Retention and Cost Model

<!-- CHANGED: New section. Explains the economic rationale for client-side evidence. -->

### Problem: Server-Side Evidence Is Unbounded

In the original design, the server stored every evidence record in Postgres
indefinitely. This creates an unbounded cost liability:
- Storage grows linearly with request volume across all tenants.
- The operator pays for audit data retention that primarily benefits the client.
- At scale, the `evidence` table dominates Postgres storage and backup costs.

### Solution: Stateless Policy Evaluator

The server is a stateless policy evaluator. It receives a request, runs OPA,
signs the result, and returns it. The only server-side state with indefinite
retention is:
- `tenants` and `api_keys` — O(number of tenants), small.
- `skills` — O(number of skills), small.

Everything else is bounded:
- `executions` — 90-day retention, O(request volume × 90 days).
- `usage_counters` — 90-day retention, O(tenants × endpoints × hours × 90 days).

### Cost Breakdown

<!-- CHANGED(v2): Recalculated row size without evidence_record_jsonb.
     Execution rows are now ~100-150 bytes instead of ~500 bytes. -->

| Data class | Retention | Storage owner | Growth |
|---|---|---|---|
| Tenants, keys | Indefinite | Server | O(tenants) — bounded |
| Skills | Indefinite | Server | O(skills) — bounded by 200/tenant cap |
| Executions | 90 days | Server | O(requests/day × 90) — bounded |
| Usage counters | 90 days | Server | O(tenants × 6 endpoints × 2160 hours) — bounded |
| Evidence records | Indefinite | **Client** | Client's problem |

At 10k requests/day across all tenants, the `executions` table holds ~900k rows
at steady state. At ~100-150 bytes/row (metadata only, no JSONB payload), that
is ~90-135 MB. Negligible on a single Postgres instance.

---

## 11. Client-Side Evidence Guide

<!-- CHANGED: New section. Guidance for clients on persisting evidence records. -->

### Overview

Every `POST /v1/validate` and `POST /v1/skills/{id}:execute` response includes
an `evidence_record` field. This is a complete, server-signed JSON object that
the client must persist to maintain an audit trail.

The server does not store evidence records. If the client discards the response,
the evidence is lost.

<!-- CHANGED(v2): Idempotent replay does not return the evidence_record.
     Clients must cache the original response. -->

**Important:** For `execute` requests with an `idempotency_key`, the server
returns the full `evidence_record` only on the **first** call. Subsequent calls
with the same key return a minimal replay response without the evidence record.
Clients must persist the evidence record from the first response.

### Option 1: Append to JSONL File

The simplest approach. Pipe the evidence record to a file:

```bash
# Using curl and jq
curl -s -X POST https://api.evidra.rest/v1/validate \
  -H "Authorization: Bearer ev1_..." \
  -H "Content-Type: application/json" \
  -d '{"actor":{"type":"agent","id":"bot","origin":"api"},"tool":"kubectl","operation":"get","params":{"target":{},"payload":{}}}' \
  | jq -c '.evidence_record' >> evidence.jsonl
```

Each line in `evidence.jsonl` is a self-contained signed record. Verify any
record later:

<!-- CHANGED(v2): Removed Authorization header from verify curl example —
     endpoint is now public. -->

```bash
# Verify a single record (no API key required)
head -1 evidence.jsonl | \
  curl -s -X POST https://api.evidra.rest/v1/evidence/verify \
    -H "Content-Type: application/json" \
    -d "{\"evidence_record\": $(cat)}"
```

### Option 2: Use the Evidra CLI Local Evidence Store

The `evidra` CLI binary includes a local evidence store with hash-linked chain
validation. Clients can write server-signed records into the local store:

```bash
# In your agent/platform code:
# 1. Call the API
response=$(curl -s -X POST https://api.evidra.rest/v1/validate ...)

# 2. Extract and store the evidence record
echo "$response" | jq -c '.evidence_record' | evidra evidence import --stdin

# 3. Verify chain integrity
evidra evidence verify
```

This gives you the best of both worlds: server-signed records (proving the
server evaluated the policy) stored in a hash-linked chain (proving no records
were deleted or reordered).

### Option 3: Send to External System

Forward evidence records to S3, Elasticsearch, Splunk, or any append-only store:

```python
# Python example
import requests, json

resp = requests.post(
    "https://api.evidra.rest/v1/validate",
    headers={"Authorization": "Bearer ev1_..."},
    json=invocation,
)
data = resp.json()

# Forward to S3
s3.put_object(
    Bucket="evidra-audit",
    Key=f"evidence/{data['event_id']}.json",
    Body=json.dumps(data["evidence_record"]),
)
```

### Verification

Clients can verify evidence records in two ways:

<!-- CHANGED(v2): Updated verification examples to use signing_payload instead
     of re-serializing JSON. Removed auth from online example. -->

**Online** — Send the record to `POST /v1/evidence/verify` (no API key
required). The server checks the signature and confirms the signing payload
matches the structured fields:

```bash
curl -s -X POST https://api.evidra.rest/v1/evidence/verify \
  -H "Content-Type: application/json" \
  -d '{"evidence_record": ...}'
```

**Offline** — Fetch the public key from `GET /v1/evidence/pubkey` once, then
verify the Ed25519 signature over the `signing_payload` string:

```go
// Go example
pubKey := fetchPublicKey() // one-time fetch from GET /v1/evidence/pubkey
rec := loadEvidenceRecord()

sigBytes, _ := base64.StdEncoding.DecodeString(rec.Signature)
valid := ed25519.Verify(pubKey, []byte(rec.SigningPayload), sigBytes)
```

```python
# Python example (using PyNaCl)
from nacl.signing import VerifyKey
import base64

verify_key = VerifyKey(public_key_bytes)
sig = base64.b64decode(record["signature"])
# signing_payload is the exact bytes that were signed — no JSON re-serialization
verify_key.verify(record["signing_payload"].encode("utf-8"), sig)
```

No JSON canonicalization needed. The `signing_payload` string is the exact
input that was signed — just verify the signature over that string.

---

## 12. Deployment

### Artifacts

- **Single binary:** `go build -o bin/evidra-api ./cmd/evidra-api`
- **Policy bundle:** The `policy/bundles/ops-v0.1/` directory is either
  embedded via `//go:embed` or mounted as a volume. Embedding is preferred for
  simplicity.
- **Signing key:** Ed25519 private key file or env var. Generated on first run
  if not provided.
- **Migrations:** SQL files in `internal/migrate/migrations/`, applied at
  startup or via CLI flag.

### Configuration (env vars)

<!-- CHANGED: Added EVIDRA_SIGNING_KEY_PATH, EVIDRA_SERVER_ID. Removed evidence-related vars. -->

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres connection string |
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address |
| `EVIDRA_API_POLICY_PATH` | no | embedded | Override policy .rego path |
| `EVIDRA_API_DATA_PATH` | no | embedded | Override data.json path |
| `EVIDRA_SIGNING_KEY_PATH` | no | `data/signing_key.pem` | Ed25519 private key file |
| `EVIDRA_SIGNING_KEY` | no | — | Ed25519 private key (base64, overrides file) |
| `EVIDRA_SERVER_ID` | no | hostname | Key ID embedded in evidence records |
| `EVIDRA_SKILLS_MAX_PER_TENANT` | no | `200` | Maximum skills per tenant |
| `EVIDRA_SKILLS_INPUT_SCHEMA_MAX_BYTES` | no | `65536` | Maximum input_schema size |
| `EVIDRA_EXECUTE_TIMEOUT_SEC` | no | `5` | Policy evaluation timeout |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | no | `json` | `json` or `text` |

### Docker

```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o evidra-api ./cmd/evidra-api

FROM alpine:3.19
COPY --from=build /app/evidra-api /usr/local/bin/
EXPOSE 8080
CMD ["evidra-api"]
```

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

### Local Dev Mode

```bash
# Start Postgres
docker run -d --name evidra-pg -e POSTGRES_DB=evidra -e POSTGRES_PASSWORD=dev -p 5432:5432 postgres:16

# Run migrations + start server (signing key auto-generated on first run)
DATABASE_URL="postgres://postgres:dev@localhost:5432/evidra?sslmode=disable" \
  bin/evidra-api --migrate
```

### Process Management

- Graceful shutdown on SIGTERM (drain in-flight requests, close DB pool).
- Health checks: `/healthz` (liveness), `/readyz` (readiness — DB reachable).
- Run as a systemd service, Docker container, or on a PaaS (Fly.io, Railway).

### Scaling Notes (Post-MVP)

- The single-binary architecture supports horizontal scaling behind a load balancer.
- All state is in Postgres. No in-process state beyond the OPA engine and
  Ed25519 key (both loaded once at startup, read-only).
- All instances must use the same Ed25519 signing key (shared via env var or
  mounted volume).
- Rate limiting moves to Redis or a shared store when running multiple instances.
- Idempotency key checks use database unique constraints, so they work across instances.

---

## 13. Traffic Measurement

### Metrics to Evaluate Demand

| Metric | Source | Query |
|---|---|---|
| **Keys issued (total)** | `tenants` table | `SELECT count(*) FROM tenants` |
| **Keys issued (last 7d)** | `tenants` table | `WHERE created_at > now() - '7d'` |
| **Active keys (used in last 7d)** | `api_keys` table | `WHERE last_used_at > now() - '7d'` |
| **Request rate per endpoint** | `usage_counters` | `SUM(request_count) GROUP BY endpoint, bucket` |
| **Deny/allow ratio** | `usage_counters` | `SUM(deny_count) / SUM(allow_count) WHERE endpoint IN ('validate','execute')` |
| **Error rate** | `usage_counters` | `SUM(error_count) / SUM(request_count)` |
| **p95 latency** | Prometheus histogram | `histogram_quantile(0.95, ...)` |
| **Top tenants by volume** | `usage_counters` | `GROUP BY tenant_id ORDER BY SUM(request_count) DESC LIMIT 20` |
| **Skills per tenant** | `skills` table | `SELECT tenant_id, count(*) FROM skills WHERE deleted_at IS NULL GROUP BY tenant_id` |
| **Executions per skill** | `executions` table | `SELECT skill_id, count(*) FROM executions GROUP BY skill_id ORDER BY count DESC` |

### Retention

<!-- CHANGED: Removed evidence rows from retention table. All server data is bounded. -->

| Data | Retention |
|---|---|
| `usage_counters` rows | 90 days (daily cleanup) |
| `executions` rows | 90 days (daily cleanup) |
| Prometheus metrics | 14 days (default Prometheus retention) |
| Structured logs | 30 days (external log sink) |

### Demand Signals

- \>50 active keys → add org/team support
- \>10k requests/day → add async usage counter writes (batch insert)
- p95 latency >200ms → profile OPA evaluation, add engine caching
- \>100 keys/week issuance → add email verification or abuse detection

---

## 14. Non-Goals

- **No marketplace.** Skills are registered per-tenant via API. No discovery, search, or cross-tenant sharing.
- **No billing.** Usage counters exist but no metering, invoicing, or payment integration.
- **No complex org/teams model.** One tenant = one API key = one skill namespace. No roles, no RBAC beyond key-level access.
- **No remote policy editing UI.** Tenants use the bundled `ops-v0.1` profile. Custom bundles require backend configuration.
- **No agent framework lock-in.** The API is HTTP + JSON. No SDK required. No MCP-specific protocol in the HTTP layer.
- **No skill versioning.** Skills can be updated in place. No version history or rollback in MVP.
- **No async execution.** All policy evaluation is synchronous.
- **No server-side evidence storage.** Evidence is returned to the client. The server is stateless with respect to audit data.

---

## 15. Implementation Plan

### Task Sequence

**Phase 1: Signing infrastructure**

<!-- CHANGED: New phase — Ed25519 signing is a prerequisite for everything else. -->

<!-- CHANGED(v2): Updated task 1.1 to include SigningPayload field.
     Updated task 1.2 to include BuildSigningPayload and payload-based verify. -->

| # | Task | DoD |
|---|---|---|
| 1.1 | `internal/evidence/types.go` — `EvidenceRecord`, `DecisionRecord` structs | Structs defined with `SigningPayload` and `Signature` fields. JSON tags match spec. No `omitempty` on fields included in signing payload. |
| 1.2 | `internal/evidence/signer.go` — Ed25519 `Signer` | `BuildSigningPayload`, `Sign`, `Verify` methods. Signing payload uses deterministic text format (not JSON). Key loading from file and env var. Auto-generation on first run. Unit tests including cross-verification (build payload in test, verify matches). |
| 1.3 | `internal/evidence/builder.go` — `BuildRecord` | Builds `EvidenceRecord` from `engine.Result` + `invocation.ToolInvocation`. Unit tests. |

**Phase 2: Database schema and storage**

| # | Task | DoD |
|---|---|---|
| 2.1 | Write `001_initial.up.sql` migration | Tables: tenants, api_keys, usage_counters, skills, executions. No evidence table. No `evidence_record_jsonb` column. Indexes verified. `down.sql` reverses cleanly. |
| 2.2 | `internal/storage/postgres.go` | Connection pool init, ping. |
| 2.3 | `internal/storage/tenants.go` — `TenantRepo` | `Create`. Unit tests with test DB. |
| 2.4 | `internal/storage/apikeys.go` — `APIKeyRepo` | `Create`, `FindByHash`, `Revoke`, `TouchLastUsed`. Unit tests. |
| 2.5 | `internal/storage/skills.go` — `SkillRepo` | `Create`, `FindByID`, `FindByName`, `ListByTenant`, `Update`, `SoftDelete`. Unit tests. |
| 2.6 | `internal/storage/executions.go` — `ExecutionRepo` | `Create`, `FindByID`, `FindByIdempotencyKey`. Unit tests. |
| 2.7 | `internal/storage/usage.go` — `UsageRepo` | `Increment` (upsert). Unit tests. |

**Phase 3: Auth and engine**

| # | Task | DoD |
|---|---|---|
| 3.1 | `internal/auth/` | Key generation, SHA-256 hashing, parsing, Bearer extraction, tenant context. Unit tests. |
| 3.2 | `internal/engine/adapter.go` | Init `runtime.Evaluator` once, expose `Evaluate` method. Unit tests. |

**Phase 4: Skills validation and invocation builder**

| # | Task | DoD |
|---|---|---|
| 4.1 | Add JSON Schema validation library | `github.com/santhosh-tekuri/jsonschema/v5` or equivalent. |
| 4.2 | `internal/skills/validator.go` | `ValidateSchema(schema)`, `ValidateInput(schema, input)`. Unit tests with edge cases. |
| 4.3 | `internal/skills/builder.go` | `BuildInvocation(skill, input, actor, env) → ToolInvocation`. Target merging, risk tag injection, context construction. Unit tests. |

**Phase 5: HTTP handlers**

| # | Task | DoD |
|---|---|---|
| 5.1 | `internal/api/keys_handler.go` | `POST /v1/keys`. Handler tests. |
| 5.2 | `internal/api/validate_handler.go` | `POST /v1/validate`. Returns signed evidence record with `signing_payload`. Handler tests. |
| 5.3 | `internal/api/skills_handler.go` | `POST/GET /v1/skills`, `GET /v1/skills/{id}`. Handler tests. |
| 5.4 | `internal/api/execute_handler.go` | `POST /v1/skills/{id}:simulate`, `POST /v1/skills/{id}:execute`, `GET /v1/executions/{id}`. Idempotent replay returns minimal response (no evidence_record). Handler tests. |
| 5.5 | `internal/api/verify_handler.go` | `POST /v1/evidence/verify` (public, no auth), `GET /v1/evidence/pubkey`. Verify checks signature over `signing_payload` and confirms payload matches structured fields. Handler tests. |
| 5.6 | Wire into router + middleware | Mount all handlers in `router.go`. Verify handler mounted without auth middleware. Rate limiter, logging, recovery, body limit. |

**Phase 6: Integration tests**

| # | Task | DoD |
|---|---|---|
| 6.1 | Validate flow integration test | `POST /v1/validate` → verify response has signed evidence_record with `signing_payload` → verify signature with pubkey endpoint. |
| 6.2 | Skill CRUD integration test | Register → list → get → update → soft delete. Against real Postgres. |
| 6.3 | Execute flow integration test | Register skill → execute (allow) → verify execution record → verify evidence_record signature via `signing_payload`. |
| 6.4 | Deny flow integration test | Register skill with target kube-system → execute → verify denied → verify evidence_record still returned. |
| 6.5 | Simulate flow integration test | Register skill → simulate → verify no evidence_record in response → verify no execution record. |
| 6.6 | Idempotency test | Execute with key → re-execute with same key → verify same execution_id → verify replay response has `idempotent_replay: true` and no evidence_record → verify single execution row in DB. |
| 6.7 | Tenant isolation test | Tenant A registers skill → Tenant B cannot see/execute it. |
| 6.8 | Evidence verify test (public) | Take evidence_record from validate response → POST to /v1/evidence/verify **without API key** → valid. Tamper with a field → verify → `payload_field_mismatch`. Tamper with signature → verify → `signature_mismatch`. |
| 6.9 | Cross-language signing payload test | Reconstruct `signing_payload` from evidence_record fields in a test (simulating a non-Go client), compare to the `signing_payload` in the response. Must match exactly. |

**Phase 7: Docker smoke test**

| # | Task | DoD |
|---|---|---|
| 7.1 | `docker-compose.yml` | `evidra-api` + `postgres:16`. Health check passes. |
| 7.2 | Smoke test script | Issue key → validate → verify evidence signature (no API key for verify) → register skill → simulate → execute → get execution → verify evidence. All pass. |

### Test Plan

| Layer | Tool | Coverage target |
|---|---|---|
| Unit | `go test` | Signer (signing payload construction, sign/verify round-trip), builder, storage repos, schema validator, invocation builder, handler logic |
| Integration | `go test` with test DB | Full request flow through HTTP → engine → signer → storage |
| Docker smoke | Shell script | End-to-end binary + Postgres, validates API contract |
| Race detection | `go test -race` | All packages |

### DoD Checklist (per task)

- [ ] Code written and compiles
- [ ] Unit tests pass
- [ ] Integration tests pass (where applicable)
- [ ] `go test -race` passes
- [ ] `golangci-lint run` clean
- [ ] No new `TODO` comments without linked issue
- [ ] Structured logging follows redaction rules
- [ ] Error responses use standard error model
- [ ] Evidence records use `signing_payload` for signatures, never `json.Marshal`
- [ ] Evidence records are never stored server-side
- [ ] Public endpoints (`/v1/evidence/verify`, `/v1/evidence/pubkey`) have no auth middleware