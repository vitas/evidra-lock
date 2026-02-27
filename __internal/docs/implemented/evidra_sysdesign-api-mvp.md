# Evidra API — MVP Architecture

**Date:** 2026-02-25
**Status:** Draft
**Scope:** Hosted API — stateless policy evaluator, client-side evidence

### Launch Phases

| | Phase 0 (MCP / single-user) | Phase 1 (Public launch) | Phase 2 (Feature flag) |
|---|---|---|---|
| **Mode** | Stateless — no Postgres | Postgres required | Postgres required |
| **Endpoints** | `POST /v1/validate`, `GET /v1/evidence/pubkey`, `GET /healthz` | + `POST /v1/keys`, `GET /readyz` | + `/v1/skills/*`, `/v1/executions/*` |
| **Auth** | Static token (`EVIDRA_API_KEY`) | Dynamic key issuance | Dynamic key issuance |
| **Usage tracking** | None (Prometheus metrics only) | `usage_counters` table | + `executions` table |
| **Landing page** | No | Yes | — |
| **Gate** | `DATABASE_URL` not set | `DATABASE_URL` set | + `EVIDRA_SKILLS_ENABLED=true` |

**Phase 0** is for running Evidra as a remote MCP server or single-tenant
sidecar. No database, no key management, no usage counters. One static API key
from an env var. The server starts instantly, evaluates policy, signs evidence,
returns it. Nothing is stored anywhere — metrics go to Prometheus, evidence goes
to the client.

**Transition to Phase 1:** Set `DATABASE_URL` and run migrations. The server
detects Postgres and enables dynamic key issuance, usage tracking, and the
landing page. The static `EVIDRA_API_KEY` continues to work (treated as a
pre-provisioned key) so existing MCP clients don't break.

All skills endpoints return `404` when the feature flag is off. The DB migration
creates all tables upfront (no schema change needed to enable Phase 2).

---

## 1. High-Level Architecture

```
                          ┌───────────────────────┐
                          │  https://evidra.samebits.com   │
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

**API service (`evidra-api`)** — A single Go binary. Core: OPA engine (loaded
once at startup from bundled policy profile) + Ed25519 signer. Two modes:

- **Phase 0 (stateless):** No Postgres. Auth via static `EVIDRA_API_KEY` env
  var. No key issuance, no usage counters, no landing page. Pure policy
  evaluator + signer. Start with one env var and a signing key.
- **Phase 1+ (Postgres):** Dynamic key issuance, usage tracking, landing page.
  Static `EVIDRA_API_KEY` continues to work alongside dynamic keys.

**Landing page** *(Phase 1+)* — Static HTML, "Get API Key" form → `POST /v1/keys`.
Disabled when `DATABASE_URL` is not set.

**PostgreSQL** *(Phase 1+)* — Single instance. Stores tenants, hashed API keys,
usage counters, skill definitions, and lightweight execution records. No read
replicas for MVP. **No evidence records** — evidence is returned to the client
in the response body for client-side storage.

**Evidence Signer** — An Ed25519 signing module (`internal/evidence`). Every
`validate` and `execute` response includes a complete, server-signed
`evidence_record` that the client persists wherever they want. The server
publishes the public key at `GET /v1/evidence/pubkey` so anyone can verify a
record's authenticity without contacting the server.

### Request Flows

**A) Issue Key Flow** *(Phase 1+ only — requires Postgres)*
```
Browser → POST /v1/keys {label?}
  1. Generate tenant_id (ULID)
  2. Generate API key: "ev1_" + 32 crypto/rand bytes (base62), 48 chars total
  3. Hash key with SHA-256 (see §4 for rationale)
  4. INSERT into tenants (id, created_at)
  5. INSERT into api_keys (id, tenant_id, key_hash, prefix, label, created_at)
  6. Return {key: "ev1_...", prefix: "ev1_...<first 8>", tenant_id} — plaintext shown once
```
Returns `404` in Phase 0 (no Postgres).

**B) Validate Flow**
```
Agent → POST /v1/validate  [Authorization: Bearer <key>]
  1. Auth middleware:
     Phase 0: constant-time compare against EVIDRA_API_KEY
       → set synthetic tenant_id = "static" in context
     Phase 1+: SHA-256(key), lookup api_keys WHERE key_hash = $1
       → reject if not found or revoked_at IS NOT NULL
       → set tenant_id in request context
  2. Handler: unmarshal body as invocation.ToolInvocation
  3. Adapter: call engine.Evaluate(ctx, inv) → Decision
  4. Build evidence record (event_id, timestamp, actor, decision, input_hash, ...)
  5. Build signing payload (deterministic text), sign with Ed25519
  6. If Postgres available: increment usage counter, update last_used_at
  7. Return {ok, event_id, decision, evidence_record (signed)}
```

**C) Evidence Verify Flow** *(opt-in, `EVIDRA_VERIFY_ENABLED=true`)*
```
Client → POST /v1/evidence/verify  [Authorization: Bearer ev1_...]
  1. If EVIDRA_VERIFY_ENABLED=false → 404
  2. Auth middleware → tenant_id
  3. Unmarshal submitted evidence_record from request body
  4. Extract signing_payload and signature from record
  5. Verify Ed25519 signature over signing_payload
  6. Re-derive signing_payload from structured fields, compare (payload_field_mismatch check)
  7. Return {ok, valid: true/false, reason}
```

Note: Offline verification via the public key (`GET /v1/evidence/pubkey`) is
the primary verification path. This endpoint is a convenience for clients that
prefer not to implement Ed25519 locally.

**D) Skills Execute Flow** *(Phase 2)*
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

**Gating (all controlled via env vars):**

| Env var | Effect |
|---|---|
| `EVIDRA_SELF_SERVE_KEYS=false` | Endpoint returns `403`. Keys issued manually. P0 default. |
| `EVIDRA_SELF_SERVE_KEYS=true` | Endpoint active. Rate-limited to 3/hour per IP. |
| `EVIDRA_INVITE_SECRET=<token>` | When set, requires `X-Invite-Token: <token>` header. |

**Request:**
```json
{"label": "my-ci-pipeline"}
```
`label` is optional, max 128 chars. Empty body is valid.

**Headers (when invite required):**
```
X-Invite-Token: <invite-secret>
```

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
- `403` — self-serve disabled, or invite token missing/invalid
- `429` — rate limit exceeded (3 keys/hour/IP)
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
  "event_id": "evt_01JEXAMPLEV",
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
    "event_id": "evt_01JEXAMPLEV",
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
    "signing_payload": "evidra.v1\nevent_id=evt_01JEXAMPLEV\ntimestamp=2026-02-25T10:00:00Z\nserver_id=evidra-api-01\npolicy_ref=sha256:abc123...\nskill_id=\nexecution_id=\nactor_type=agent\nactor_id=deploy-bot\nactor_origin=api\ntool=terraform\noperation=apply\nenvironment=\ninput_hash=sha256:def456...\nallow=true\nrisk_level=low\nreason=all_policies_passed\nreasons=\nrule_ids=\nhints=",
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

**Design note: deny = HTTP 200.** Both `validate` and `execute` return `200`
for all policy decisions, including deny. The HTTP status reflects *request
processing*, not *policy outcome*. A deny is a successful evaluation, not a
server error. The policy verdict is in `decision.allow`.

Why this matters for clients:
- **Do not rely on HTTP status to detect deny.** Check `decision.allow` in
  every response. A `200` with `allow: false` means the action is blocked.
- **Monitoring:** Track deny rate via `decision.allow`, not HTTP error codes.
  Standard HTTP error rate dashboards (4xx/5xx) will not surface policy denials.
- **Proxies and SDKs:** Some HTTP clients treat only 4xx/5xx as failures. If
  your platform auto-retries on non-2xx, the `200` prevents retry storms on
  deny (which would be pointless — the policy will deny again).

Rationale: a `403`/`409` would conflate infrastructure errors with policy
decisions, make evidence records ambiguous (was it a real error or a policy
deny?), and trigger retry logic in most HTTP client libraries. Stripe,
Twilio, and other policy-evaluating APIs use the same pattern.

---

### `POST /v1/evidence/verify`

**Disabled by default.** Controlled by `EVIDRA_VERIFY_ENABLED` (default: `false`).
Returns `404` when disabled.

When enabled, requires `Authorization: Bearer <key>` (same as other
authenticated endpoints). This prevents the endpoint from becoming a public
CPU target — Ed25519 verify + payload rebuild is cheap per-call but unbounded
under bot traffic.

**Why not public:** Offline verification via `GET /v1/evidence/pubkey` covers
every use case (auditors, compliance systems, cross-language clients). The
server-side verify endpoint is a convenience, not a necessity. Making it
authenticated and opt-in eliminates a DDoS surface with zero loss of
functionality.

**Request:**
```json
{
  "evidence_record": {
    "event_id": "evt_01JEXAMPLEV",
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
    "signing_payload": "evidra.v1\nevent_id=evt_01JEXAMPLEV\n...",
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
does not match the structured fields in the evidence record. The server
re-derives the signing payload from the structured JSON fields and compares it
to the submitted `signing_payload`. If they differ, the signature may still be
mathematically valid, but the JSON fields have been tampered with after signing.
This catches attacks where an adversary modifies displayed data (e.g., changing
`allow: false` to `allow: true` in the JSON) while preserving the original
`signing_payload` and signature.

**Errors:**
- `400` — missing or malformed evidence_record
- `401` — missing or invalid key
- `404` — endpoint disabled (`EVIDRA_VERIFY_ENABLED=false`)
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

---

> **Phase 2 — all endpoints below require `EVIDRA_SKILLS_ENABLED=true`.** Returns `404` when disabled.

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

### `PUT /v1/skills/{skill_id}`

Update a skill definition. Full replacement — all fields are required (same body
as `POST /v1/skills`). Requires `Authorization: Bearer <key>`.

**Request body** — same schema as `POST /v1/skills`.

**Response (200):** Returns the updated skill object (same shape as `POST /v1/skills` response).

**Errors:**
- `400` — validation failure (missing required fields, invalid JSON schema)
- `401` — missing or invalid key
- `404` — skill not found (or belongs to another tenant)
- `409` — name conflict with another active skill belonging to this tenant

---

### `DELETE /v1/skills/{skill_id}`

Soft-delete a skill. The skill row is marked with `deleted_at` and excluded
from listings. Existing execution records referencing this skill are not
affected. Requires `Authorization: Bearer <key>`.

**Response (200):**
```json
{"ok": true}
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

**Response (200) — allowed:**
```json
{
  "ok": true,
  "execution_id": "ex_01JEXAMPLE",
  "event_id": "evt_01JEXAMPLEE",
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
    "event_id": "evt_01JEXAMPLEE",
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
    "signing_payload": "evidra.v1\nevent_id=evt_01JEXAMPLEE\n...",
    "signature": "base64-encoded-ed25519-signature-over-signing_payload"
  }
}
```

**Response (200) — denied:**
```json
{
  "ok": true,
  "execution_id": "ex_01JEXAMPLE2",
  "event_id": "evt_01JEXAMPLEE2",
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
    "event_id": "evt_01JEXAMPLEE2",
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
    "signing_payload": "evidra.v1\nevent_id=evt_01JEXAMPLEE2\n...",
    "signature": "base64-encoded-ed25519-signature-over-signing_payload"
  }
}
```

Note: a denied execution returns `200` with `"ok": true` and `"status": "denied"`.
See design note under `POST /v1/validate` for full rationale. Clients **must**
check `decision.allow` — HTTP `200` does not mean the action is permitted.

**Idempotency:** If `idempotency_key` matches an existing execution for this
tenant, a minimal replay response is returned without re-evaluation:

```json
{
  "ok": true,
  "idempotent_replay": true,
  "execution_id": "ex_01JEXAMPLE",
  "event_id": "evt_01JEXAMPLEE",
  "decision": {
    "allow": true,
    "risk_level": "low"
  },
  "status": "allowed",
  "created_at": "2026-02-25T10:00:00Z",
  "message": "This request was already processed. Use your previously received evidence_record for audit purposes."
}
```

The replay response is built from the execution row's stored metadata
(`decision_allow`, `decision_risk`, `event_id`, `status`, `created_at`). No full evidence
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
    "event_id": "evt_01JEXAMPLEE",
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

No auth. In Phase 1+: returns `200 OK` only when Postgres is reachable
(`SELECT 1`). Returns `503` otherwise. In Phase 0 (no Postgres): always returns
`200 OK` (same as `/healthz`). Used by orchestrators for readiness gating.

---

### Endpoint Comparison

| | Validate | Simulate | Execute |
|---|---|---|---|
| **Phase** | **0** | **2** | **2** |
| Endpoint | `POST /v1/validate` | `POST /v1/skills/{id}:simulate` | `POST /v1/skills/{id}:execute` |
| Input | Raw `ToolInvocation` | Skill input (schema-validated) | Skill input (schema-validated) |
| Policy evaluation | Yes | Yes | Yes |
| Evidence record returned | **Yes** (signed) | **No** | **Yes** (signed) |
| Execution record in DB | **No** | **No** | **Yes** (lightweight, 90 days) |
| Usage counter | Incremented (`validate`) | Incremented (`simulate`) | Incremented (`execute`) |
| Idempotency key | Not supported | Not supported | Supported |

`simulate` is for dry-run / pre-flight checks — rate-limited and metered, but produces no evidence record. `validate` accepts a raw `ToolInvocation` directly. `execute` is the skills-based flow with schema validation and execution records. Evidence records are produced **regardless** of allow/deny decisions. Timeouts: OPA 5s, HTTP 30s, DB 10s. The server does not retry internally; callers retry with idempotency keys.

### Rate Limits

**Per-key limits:**

| Endpoint | Limit |
|---|---|
| `POST /v1/validate` | 60 req/min |
| `POST /v1/skills` | 30 req/min |
| `PUT /v1/skills/{id}` | 30 req/min |
| `DELETE /v1/skills/{id}` | 30 req/min |
| `GET /v1/skills`, `GET /v1/skills/{id}` | 120 req/min |
| `POST /v1/skills/{id}:simulate` | 60 req/min |
| `POST /v1/skills/{id}:execute` | 60 req/min |
| `GET /v1/executions/{id}` | 120 req/min |

Burst allowance: 2x the per-minute rate.

**Per-IP limits (unauthenticated):**
- `POST /v1/keys`: 3 requests/hour per IP (+ feature gate + optional invite secret, see §Abuse Mitigations)
- `GET /v1/evidence/pubkey`: 60 requests/minute per IP
- Global: 100 requests/minute per IP across all endpoints

**Opt-in endpoints (per-key, when enabled):**
- `POST /v1/evidence/verify`: 60 requests/minute per key (requires `EVIDRA_VERIFY_ENABLED=true`)

**Implementation:** In-memory token bucket (`golang.org/x/time/rate`) keyed by
tenant_id (authenticated) or IP (unauthenticated). Sufficient for single-node
MVP. Returns `429 Too Many Requests` with `Retry-After` header.

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

### `skills` *(Phase 2)*

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

### `executions` *(Phase 2)*

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
- `event_id` is a plain reference string (e.g. `"evt_01JEXAMPLEE"`). It allows correlation with client-side evidence stores but is not a foreign key — the server does not store evidence.
- `idempotency_key` uniqueness is enforced per tenant. A daily cleanup job deletes execution rows older than 90 days, which also removes stale idempotency keys.
- Idempotent replay builds the response from the stored metadata columns (`decision_allow`, `decision_risk`, `event_id`, `status`, `created_at`). No evidence payload is cached or returned on replay.

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

The server stores no evidence — all evidence records are returned to clients.
Server-side data is bounded: at 10k req/day, `executions` holds ~900k rows
(~90-135 MB) at steady state.

| Table | Retention | Cleanup |
|---|---|---|
| `tenants` | Indefinite | — |
| `api_keys` | Indefinite (revoked keys kept for audit) | — |
| `skills` | Indefinite (soft-deleted rows cleaned after 90 days) | Daily `DELETE WHERE deleted_at < now() - '90 days'` |
| `executions` | 90 days | Daily `DELETE WHERE created_at < now() - '90 days'` |
| `usage_counters` | 90 days | Daily `DELETE WHERE bucket < now() - '90 days'` |

---

### Migrations

```
internal/migrate/migrations/
  001_initial.up.sql      -- tenants, api_keys, usage_counters, skills, executions
  001_initial.down.sql
```

---

## 4. Skill Definition Schema *(Phase 2)*

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

## 4b. Input Adapters — Extraction Layer

### Design Principle: Evidra Does Not Parse Raw Artifacts

Evidra skills accept **business parameters** (`namespace`, `destroy_count`, `image_tag`), not raw tool output (`plan.json`, `manifest.yaml`). Parsing terraform plans, kubectl manifests, ArgoCD specs, and AWS API responses is a maintenance nightmare — each tool has its own schema, versioning, and breaking changes. Evidra deliberately excludes this from its scope.

The question is: **who extracts business parameters from raw artifacts?**

| Caller type | Who parses | How |
|---|---|---|
| **AI agent** (Claude Code, Cursor, MCP) | The LLM itself | LLM reads `terraform plan -json`, understands "5 resources will be destroyed", sends `{"destroy_count": 5}` to skill. Parsing is free — it's what LLMs do. |
| **CI pipeline** (GitHub Actions, GitLab CI) | An **input adapter** | A thin script/binary that reads tool-specific output and emits Evidra skill input. Shipped as part of the integration, not Evidra core. |
| **Platform** (Backstage, Slack bot) | The platform itself | Backstage already knows the service name, namespace, environment. It is the source of data, not a consumer of raw artifacts. |

For AI agents, no adapter is needed — the LLM is the universal parser. For CI and automation, Evidra provides an **adapter interface** and a set of **reference adapters** for common tools.

### Adapter Interface

An input adapter is any function (Go, shell, Python) that conforms to:

```
Input:  raw artifact bytes + adapter-specific config
Output: JSON object matching the skill's input_schema
```

In Go:

```go
// Package adapter defines the interface for converting tool-specific
// output into Evidra skill input parameters.
package adapter

import "context"

// Result is the output of an adapter: a JSON-serializable map
// matching the target skill's input_schema.
type Result struct {
    // Input is the extracted business parameters.
    Input map[string]any `json:"input"`
    // Metadata is optional provenance info (source file, tool version, etc.)
    // It is NOT sent to the skill — it is for audit/logging only.
    Metadata map[string]any `json:"metadata,omitempty"`
}

// Adapter converts raw tool output into skill input parameters.
type Adapter interface {
    // Name returns the adapter identifier, e.g. "terraform-plan", "k8s-manifest".
    Name() string
    // Convert reads raw artifact bytes and extracts business parameters.
    // The config map allows adapter-specific settings (e.g. target environment,
    // resource type filters).
    Convert(ctx context.Context, raw []byte, config map[string]string) (*Result, error)
}
```

Key properties:

- **Adapters live outside Evidra core.** They are in a separate Go module (`github.com/evidra/adapters`) or distributed as standalone binaries. Evidra server never imports adapter code.
- **Adapters are optional.** Direct API calls with pre-assembled input work without any adapter. Adapters are a convenience for CI integrations.
- **Adapters are composable.** A GitHub Action can shell out to `evidra-adapter-terraform`, pipe the result to `curl`, and be done. No Go SDK required.
- **Adapters are versioned independently.** When terraform changes its JSON schema, only the terraform adapter updates. Evidra core is unaffected.

### Reference Adapter: `terraform-plan`

Uses HashiCorp's official `github.com/hashicorp/terraform-json` library — the stable, de-coupled representation of `terraform show -json` output. This library explicitly supports external consumers and follows terraform's 1.x compatibility promises.

```go
package terraform

import (
    "context"
    "encoding/json"
    "fmt"

    tfjson "github.com/hashicorp/terraform-json"
    "github.com/evidra/adapters/adapter"
)

type PlanAdapter struct{}

func (a *PlanAdapter) Name() string { return "terraform-plan" }

func (a *PlanAdapter) Convert(
    ctx context.Context, raw []byte, config map[string]string,
) (*adapter.Result, error) {
    var plan tfjson.Plan
    if err := json.Unmarshal(raw, &plan); err != nil {
        return nil, fmt.Errorf("parse terraform plan: %w", err)
    }

    var creates, updates, deletes, replaces int
    resourceTypes := map[string]bool{}
    for _, rc := range plan.ResourceChanges {
        if rc.Change == nil {
            continue
        }
        resourceTypes[rc.Type] = true
        for _, action := range rc.Change.Actions {
            switch action {
            case tfjson.ActionCreate:
                creates++
            case tfjson.ActionUpdate:
                updates++
            case tfjson.ActionDelete:
                deletes++
            }
        }
        if rc.Change.Actions.Replace() {
            replaces++
        }
    }

    types := make([]string, 0, len(resourceTypes))
    for t := range resourceTypes {
        types = append(types, t)
    }

    return &adapter.Result{
        Input: map[string]any{
            "create_count":   creates,
            "update_count":   updates,
            "destroy_count":  deletes,
            "replace_count":  replaces,
            "resource_types": types,
            "total_changes":  creates + updates + deletes + replaces,
        },
        Metadata: map[string]any{
            "terraform_version": plan.TerraformVersion,
            "format_version":    plan.FormatVersion,
            "resource_count":    len(plan.ResourceChanges),
        },
    }, nil
}
```

Usage in a GitHub Action:

```bash
# 1. Generate plan JSON (standard terraform workflow)
terraform plan -out=tfplan.bin
terraform show -json tfplan.bin > tfplan.json

# 2. Adapter extracts business parameters
SKILL_INPUT=$(evidra-adapter-terraform < tfplan.json)
# → {"create_count":2,"destroy_count":0,"update_count":1,...}

# 3. Call Evidra skill
curl -X POST https://api.evidra.dev/v1/skills/terraform-apply:execute \
  -H "Authorization: Bearer $EVIDRA_API_KEY" \
  -d "{\"actor\":{\"type\":\"pipeline\",\"id\":\"$GITHUB_RUN_ID\"},\"input\":$SKILL_INPUT}"
```

### Planned Adapters

| Adapter | Source artifact | Library / approach | Status |
|---|---|---|---|
| `terraform-plan` | `terraform show -json` output | `hashicorp/terraform-json` (Go) — official, stable | Reference impl |
| `k8s-manifest` | YAML/JSON manifest | `k8s.io/apimachinery` unmarshalling | Planned |
| `k8s-admission` | AdmissionReview JSON | `k8s.io/api/admission/v1` | Planned |
| `argocd-app` | ArgoCD Application spec | JSON path extraction | Planned |
| `aws-cloudtrail` | CloudTrail event JSON | JSON path extraction | Planned |
| `generic-json` | Any JSON | JSONPath / jq expressions via config | Planned |

The `generic-json` adapter uses configurable JSONPath expressions — no code required. For tool-specific needs, a custom adapter is ~50 lines of Go.

### AI Agents as Universal Adapters

For AI-driven workflows (MCP, Claude Code, Cursor, Windsurf), **the LLM replaces all adapters**. The agent:

1. Runs `terraform plan -json` (or kubectl, or aws cli)
2. Reads the output natively — LLMs parse JSON/YAML without libraries
3. Extracts the relevant parameters in natural language reasoning
4. Calls the Evidra skill with clean business input

This is not a workaround — it is the **primary use case**. AI agents are better at parsing heterogeneous, versioned, semi-structured output than static code. They handle schema changes, new resource types, and edge cases without adapter updates.

For CI pipelines (no LLM in the loop), static adapters fill the same role. Both paths produce identical skill input — the policy engine sees no difference.

### Adapter Distribution

Adapters are distributed as:

- **Go module** (`github.com/evidra/adapters`) — importable for Go integrations
- **Standalone binaries** — `evidra-adapter-terraform`, `evidra-adapter-k8s` — stdin/stdout, usable from any language or shell
- **Container images** — for CI steps that prefer `docker run evidra/adapter-terraform < plan.json`

All adapters are optional, external to Evidra core, and versioned independently. Breaking changes in a tool's output format are fixed in the adapter, not in Evidra.

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

### API Key Implementation Notes

Authentication hashes the incoming Bearer token with SHA-256 and queries Postgres by `key_hash`. On miss, a constant-time sleep (50-100ms jitter) prevents timing-based enumeration. Key parsing requires `Bearer ` prefix, `ev1_` key prefix, length 40-60 chars, base62 characters only. The plaintext key is generated in memory, returned once in the response with `Cache-Control: no-store`, and never persisted — logs record only the `prefix` field.

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
// 1. Load Ed25519 private key from EVIDRA_SIGNING_KEY (base64 env var)
//    or EVIDRA_SIGNING_KEY_PATH (PEM file on disk, read-only)
// 2. If neither is set:
//    a) EVIDRA_ENV=development → generate in-memory ephemeral keypair
//       log.Warn("ephemeral signing key — not persisted, evidence
//                 will not survive restart, do not use in production")
//    b) any other EVIDRA_ENV (or unset) → log.Fatal + os.Exit(1)
//       "EVIDRA_SIGNING_KEY or EVIDRA_SIGNING_KEY_PATH required"
// 3. Derive public key from private key
// 4. Set server_id (key_id) from EVIDRA_SERVER_ID env var or hostname
//
// The server NEVER writes key material to disk. In dev mode, the key lives
// only in process memory and is lost on restart. This is intentional.

type Signer struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
    keyID      string  // e.g. "evidra-api-01"
}
```

**Why no auto-generation in production:** On ephemeral disks (k8s pods, PaaS),
a restart or reschedule would generate a new key. All previously issued evidence
records become unverifiable — the public key at `/v1/evidence/pubkey` no longer
matches their signatures. This silently destroys the core value proposition.

**Why no write-to-disk even in dev:** Writing a generated key to
`data/signing_key.pem` creates a false sense of persistence. The developer
restarts, the key survives, they assume this is how production works. Then
they deploy to k8s, the pod gets rescheduled, and the key is gone. Ephemeral
means ephemeral — if you need persistence, provide the key explicitly.

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
reasons={length-prefixed, sorted alphabetically}\n
rule_ids={length-prefixed, sorted alphabetically}\n
hints={length-prefixed, sorted alphabetically}\n
```

Rules:
- Version prefix `evidra.v1\n` is always the first line.
- All fields are present in every payload (empty string for absent values).
- Timestamps are always RFC3339 in UTC with `Z` suffix (no timezone offset).
- No trailing newline after the last field.

**Scalar values** are inserted verbatim — no sanitization, no escaping. Instead,
the server **rejects** values containing `\n` or `\r` at input validation time
(`ValidateStructure` returns 400). This is enforced for all string fields that
appear in the signing payload: `actor.type`, `actor.id`, `actor.origin`, `tool`,
`operation`, `environment`, `reason`. These fields are short identifiers — a
newline in them is always a bug, never legitimate content.

**List values** (`reasons`, `rule_ids`, `hints`) use length-prefixed encoding
to preserve original bytes without delimiter ambiguity:

```
hints=3:foo,11:hello,world,0:
```

Each item is `len:value` where `len` is the byte length of `value` (decimal).
Items are sorted alphabetically by their original value (before prefixing),
then joined with `,`. Empty list → empty string after `=`. Empty item → `0:`.

This encoding is unambiguous regardless of what characters appear in the values
(commas, newlines, unicode — all preserved). A verifier splits on `,`, parses
`len:` prefix, reads exactly `len` bytes, and recovers the original values.

```go
// lengthPrefixedJoin encodes a list as sorted, length-prefixed items.
func lengthPrefixedJoin(ss []string) string {
    if len(ss) == 0 {
        return ""
    }
    sorted := make([]string, len(ss))
    copy(sorted, ss)
    sort.Strings(sorted)
    parts := make([]string, len(sorted))
    for i, s := range sorted {
        parts[i] = fmt.Sprintf("%d:%s", len(s), s)
    }
    return strings.Join(parts, ",")
}

// parseLengthPrefixed recovers original values from the encoded form.
func parseLengthPrefixed(encoded string) ([]string, error) {
    if encoded == "" {
        return nil, nil
    }
    var result []string
    for len(encoded) > 0 {
        colonIdx := strings.IndexByte(encoded, ':')
        if colonIdx < 0 {
            return nil, fmt.Errorf("missing length prefix")
        }
        n, err := strconv.Atoi(encoded[:colonIdx])
        if err != nil || n < 0 {
            return nil, fmt.Errorf("invalid length: %q", encoded[:colonIdx])
        }
        start := colonIdx + 1
        if start+n > len(encoded) {
            return nil, fmt.Errorf("value truncated")
        }
        result = append(result, encoded[start:start+n])
        encoded = encoded[start+n:]
        if len(encoded) > 0 {
            if encoded[0] != ',' {
                return nil, fmt.Errorf("expected comma separator")
            }
            encoded = encoded[1:]
        }
    }
    return result, nil
}
```

This format is constructible in any language in ~15 lines. No ambiguity, no
lossy transforms, no semantic mismatch between JSON fields and signed payload.

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
    fmt.Fprintf(&b, "reasons=%s\n", lengthPrefixedJoin(rec.Decision.Reasons))
    fmt.Fprintf(&b, "rule_ids=%s\n", lengthPrefixedJoin(rec.Decision.RuleIDs))
    fmt.Fprintf(&b, "hints=%s", lengthPrefixedJoin(rec.Decision.Hints))
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

```

**Key rotation:**

MVP uses a single key. For rotation (post-MVP):
1. Generate new keypair.
2. Publish both old and new public keys at `/v1/evidence/pubkey` (returns array).
3. New signatures use the new key. Old signatures remain verifiable with the old public key.
4. Each evidence record's `server_id` identifies which key signed it.

### Evidence Record Schema

The `evidence_record` is a self-contained JSON object:

```go
type EvidenceRecord struct {
    EventID        string          `json:"event_id"`         // "evt_" + ULID
    Timestamp      time.Time       `json:"timestamp"`        // UTC
    ServerID       string          `json:"server_id"`        // key_id of signing key
    PolicyRef      string          `json:"policy_ref"`       // SHA-256 of policy bundle
    SkillID        string          `json:"skill_id"`         // set for execute, "" for validate
    ExecutionID    string          `json:"execution_id"`     // set for execute, "" for validate
    Actor          invocation.Actor `json:"actor"`
    Tool           string          `json:"tool"`
    Operation      string          `json:"operation"`
    Environment    string          `json:"environment"`
    InputHash      string          `json:"input_hash"`       // SHA-256 of server-canonical input (not client-reproducible)
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

| **Replay of evidence records** | Each record has a unique `event_id` (ULID). Clients deduplicate by `event_id`. |
| **Input injection via skill input** | Input validated against registered `input_schema` before `ToolInvocation` construction. Schema is set at registration time, not by the caller. |
| **Risk tag escalation** | `risk_tags` are fixed in the skill definition. Execute request cannot override them. |
| **Replay of API requests** | `idempotency_key` deduplication. TLS required for all external traffic. |
| **Privilege escalation via skill mutation** | Skills can only be mutated by the same tenant's API key. No cross-tenant access. |
| **SSRF** | The server does not make outbound HTTP calls. It only evaluates policy locally. |
| **Signing key compromise** | Key is loaded from env var or file, never logged, never exposed via API. Server refuses to start without an explicit key in production (no auto-generation). Rotation procedure documented. |
| **Enumeration** | 404 returned for all not-found cases regardless of existence. IDs use ULIDs (not sequential). |
| **Verify endpoint abuse** | `POST /v1/evidence/verify` is disabled by default (`EVIDRA_VERIFY_ENABLED=false`). When enabled, requires API key authentication. Offline verification via public key is the primary path. |

### Abuse Mitigations

**`POST /v1/keys` — the highest-risk endpoint (mints credentials, no auth required):**

- **Feature gate:** `EVIDRA_SELF_SERVE_KEYS` env var. When `false` (P0 default), endpoint returns `403`. Keys issued manually only. Zero attack surface.
- **Invite secret:** `EVIDRA_INVITE_SECRET` env var. When set, `POST /v1/keys` requires `X-Invite-Token` header matching the secret. Transforms the endpoint from public to gated. Share token with early beta users, rotate when needed.
- **Per-IP rate limit:** 3 requests/hour per IP (stricter than Traefik's blanket limit). In-memory token bucket, separate from per-key rate limits.
- **Phase matrix:**
  - P0 private: `SELF_SERVE_KEYS=false` — endpoint disabled
  - P0 beta: `SELF_SERVE_KEYS=true` + `INVITE_SECRET=<token>` — gated + rate limited
  - P1 public: `SELF_SERVE_KEYS=true` + no invite — open + rate limited + CAPTCHA

**General abuse controls:**

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

## 6. Reuse of Existing Engine

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

### Building and signing evidence

After `Evaluate` returns, the handler builds an evidence record in memory:

```go
rec := evidence.EvidenceRecord{
    EventID:     "evt_" + ulid.Make().String(),
    Timestamp:   time.Now().UTC(),
    PolicyRef:   decision.PolicyRef,
    Actor:       inv.Actor,
    Tool:        inv.Tool,
    Operation:   inv.Operation,
    Environment: inv.Environment,
    InputHash:   sha256OfInput(inv),
```

**`input_hash` semantics:** The hash is computed server-side over the
`ToolInvocation` struct after `ValidateStructure()` has normalized it. It is
an **opaque, server-internal hash** — clients must not attempt to reproduce it.

The hash is stable across requests to the **same server version**: identical
`ToolInvocation` input produces identical hash. It is **not stable** across
server versions (a Go upgrade, struct field addition, or serialization change
may alter it) and **not reproducible** from other languages. This is acceptable
because the hash serves only two purposes:
1. Detecting duplicate inputs within a single server deployment.
2. Proving that two evidence records evaluated byte-identical input (same
   server version assumed).

Clients comparing inputs should use the `event_id` for correlation, not
`input_hash`. If cross-version or cross-language input comparison is needed
in the future, `input_hash` should be replaced with a hash over a
deterministic canonical form (similar to `signing_payload`). This is a
post-MVP concern.

```go
func sha256OfInput(inv invocation.ToolInvocation) string {
    // Implementation detail: json.Marshal produces deterministic output
    // for Go structs (fixed field order). Map key order is de facto sorted
    // in current Go versions but this is NOT a language guarantee.
    // Treat this hash as opaque — do not depend on cross-version stability.
    b, _ := json.Marshal(inv)
    h := sha256.Sum256(b)
    return "sha256:" + hex.EncodeToString(h[:])
}
```
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

## 7. Code Architecture (Go)

```
cmd/
  evidra-api/
    main.go                  # Entrypoint: detect mode (Phase 0 if no DATABASE_URL),
                             # init DB if available, init engine, init signer,
                             # init auth (static key or DB-backed), start server

internal/
  api/
    router.go                # stdlib http.ServeMux, mounts handlers
    middleware.go            # Request logging, recovery, request-id, CORS, body limit
    keys_handler.go          # POST /v1/keys
    validate_handler.go      # POST /v1/validate
    skills_handler.go        # POST/GET /v1/skills, GET/PUT/DELETE /v1/skills/{id}  [Phase 2]
    execute_handler.go       # POST /v1/skills/{id}:simulate, POST /v1/skills/{id}:execute,  [Phase 2]
                             # GET /v1/executions/{id}
    verify_handler.go        # POST /v1/evidence/verify (opt-in, auth required), GET /v1/evidence/pubkey
    health_handler.go        # GET /healthz, /readyz
    response.go              # JSON response helpers, error formatting

  auth/
    middleware.go            # Bearer token extraction, dual-mode auth:
                             #   Phase 0: constant-time compare against EVIDRA_API_KEY
                             #   Phase 1+: SHA-256 hash → DB lookup
                             # Tenant context injection (synthetic "static" tenant in Phase 0)
    apikey.go                # Key generation, parsing, hashing functions
    context.go               # TenantID get/set on context.Context

  engine/
    adapter.go               # Thin wrapper: calls pkg/runtime.Evaluator directly
                             # Manages OPA engine lifecycle (init once at startup)

  evidence/
    signer.go                # Ed25519 signing and verification, signing payload builder
    builder.go               # Build EvidenceRecord from Decision + ToolInvocation
    types.go                 # EvidenceRecord, DecisionRecord structs

  skills/                            # [Phase 2]
    validator.go             # JSON Schema validation (registration + input)
    builder.go               # BuildInvocation(skill, input, actor, env) → ToolInvocation

  storage/
    postgres.go              # *sql.DB initialization, connection pool, ping
    tenants.go               # TenantRepo: Create
    apikeys.go               # APIKeyRepo: Create, FindByHash, Revoke, TouchLastUsed
    skills.go                # SkillRepo: [Phase 2] Create, FindByID, FindByName, ListByTenant,
                             #   Update, SoftDelete
    executions.go            # ExecutionRepo: [Phase 2] Create, FindByID, FindByIdempotencyKey
    usage.go                 # UsageRepo: Increment (upsert usage_counters)

  ratelimit/
    limiter.go               # Token bucket per key/IP, cleanup goroutine

  migrate/

    migrations/
      001_initial.up.sql     # CREATE TABLE tenants, api_keys, usage_counters,
                             #   skills, executions
      001_initial.down.sql

# Separate module: github.com/evidra/adapters (NOT imported by server)
adapters/
  adapter/
    adapter.go               # Adapter interface, Result type
  terraform/
    plan.go                  # PlanAdapter: terraform show -json → skill input
  k8s/
    manifest.go              # ManifestAdapter: k8s YAML/JSON → skill input  [Planned]
    admission.go             # AdmissionAdapter: AdmissionReview → skill input  [Planned]
  generic/
    jsonpath.go              # Generic JSONPath adapter via config  [Planned]
  cmd/
    evidra-adapter-terraform/ # Standalone binary: stdin → stdout
      main.go
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

adapters/*
  → (NO dependency on internal/ or pkg/ — entirely separate Go module)
  → external libs only (hashicorp/terraform-json, k8s.io/apimachinery, etc.)
```

**Key boundary:** `internal/engine` and `internal/evidence` are the only
packages that import `pkg/*`. Both are thin adapters — no business logic.

---

## 8. Observability

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
  "event_id": "evt_01JEXAMPLEE",
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

- `validate` and `execute` calls return signed evidence records. This is the
  primary audit mechanism — the client persists the records.
- `simulate` calls are counted via `usage_counters` but produce no evidence.
- Server-side `usage_counters` provide aggregate analytics (request counts,
  allow/deny ratios, latency) without storing per-request audit data.

---

## 9. Client-Side Evidence Guide

Every `POST /v1/validate` and `POST /v1/skills/{id}:execute` response includes
an `evidence_record` field — a complete, server-signed JSON object. The server
does not store evidence records; the client must persist them.

For `execute` with `idempotency_key`, the full `evidence_record` is returned
only on the first call. Subsequent calls return a minimal replay response.

Storage options: append to JSONL file, use the `evidra` CLI local evidence
store (hash-linked chains), or forward to S3/ELK/Splunk. Verification is done
offline using the Ed25519 public key from `GET /v1/evidence/pubkey` (the
primary path). The optional `POST /v1/evidence/verify` endpoint is available
when `EVIDRA_VERIFY_ENABLED=true`.

See `docs/client-evidence-guide.md` for curl/Python examples and full details.

---

## 10. Deployment

### Artifacts

- **Single binary:** `go build -o bin/evidra-api ./cmd/evidra-api`
- **Policy bundle:** The `policy/bundles/ops-v0.1/` directory is either
  embedded via `//go:embed` or mounted as a volume. Embedding is preferred for
  simplicity.
- **Signing key:** Ed25519 private key, provided via env var or file path.
  Required in production — server refuses to start without it. In dev mode
  (`EVIDRA_ENV=development`), an ephemeral key is auto-generated with a warning.
- **Migrations:** SQL files in `internal/migrate/migrations/`, applied at
  startup or via CLI flag.

### Configuration (env vars)

| Variable | Required | Default | Description |
|---|---|---|---|
| `EVIDRA_SKILLS_ENABLED` | No | `false` | Enable Phase 2 skills endpoints. When `false`, all `/v1/skills/*` and `/v1/executions/*` return `404`. |
|---|---|---|---|
| `EVIDRA_VERIFY_ENABLED` | No | `false` | Enable `POST /v1/evidence/verify`. When `false`, returns `404`. Offline verification via pubkey is always available. |
|---|---|---|---|
| `DATABASE_URL` | **Phase 1+** | — | Postgres connection string. When absent, server runs in Phase 0 (stateless). |
| `EVIDRA_API_KEY` | **Phase 0** | — | Static API key for single-tenant mode. Any string ≥32 chars. Required when `DATABASE_URL` is not set. Also works in Phase 1+ as a pre-provisioned key (bypasses DB lookup). |
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address |
| `EVIDRA_API_POLICY_PATH` | no | embedded | Override policy .rego path |
| `EVIDRA_API_DATA_PATH` | no | embedded | Override data.json path |
| `EVIDRA_ENV` | no | `production` | `production` or `development`. Controls signing key behavior. |
| `EVIDRA_SIGNING_KEY` | **yes*** | — | Ed25519 private key (base64). Required in production. |
| `EVIDRA_SIGNING_KEY_PATH` | **yes*** | — | Ed25519 private key PEM file. Alternative to env var. |

\* One of `EVIDRA_SIGNING_KEY` or `EVIDRA_SIGNING_KEY_PATH` is **required** when `EVIDRA_ENV != development`. Server refuses to start without it.
| `EVIDRA_SERVER_ID` | no | hostname | Key ID embedded in evidence records |
| `EVIDRA_SKILLS_MAX_PER_TENANT` | no | `200` | Maximum skills per tenant |
| `EVIDRA_SKILLS_INPUT_SCHEMA_MAX_BYTES` | no | `65536` | Maximum input_schema size |
| `EVIDRA_EXECUTE_TIMEOUT_SEC` | no | `5` | Policy evaluation timeout |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | no | `json` | `json` or `text` |

### Docker

```dockerfile
FROM golang:1.24.6-alpine AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o evidra-api ./cmd/evidra-api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /app/evidra-api /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/evidra-api"]
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

**Phase 0 (no Postgres — fastest start):**
```bash
# One-liner: stateless mode with ephemeral signing key
EVIDRA_ENV=development \
EVIDRA_API_KEY="my-dev-token-at-least-32-characters-long" \
  bin/evidra-api
```

No database, no migrations. Validate endpoint works immediately.

**Phase 1+ (with Postgres):**
```bash
# Start Postgres
docker run -d --name evidra-pg -e POSTGRES_DB=evidra -e POSTGRES_PASSWORD=dev -p 5432:5432 postgres:16

# Run migrations + start server (dev mode: ephemeral signing key)
EVIDRA_ENV=development \
DATABASE_URL="postgres://postgres:dev@localhost:5432/evidra?sslmode=disable" \
  bin/evidra-api --migrate
```

### Process Management

- Graceful shutdown on SIGTERM (drain in-flight requests; close DB pool if connected).
- Health checks: `/healthz` (liveness — process + OPA), `/readyz` (readiness — DB if connected, else same as healthz).
- Phase 0: no external dependencies. Start time < 1 second.
- Run as a systemd service, Docker container, sidecar, or on a PaaS (Fly.io, Railway).

### Scaling Notes (Post-MVP)

- The single-binary architecture supports horizontal scaling behind a load balancer.
- All state is in Postgres. No in-process state beyond the OPA engine and
  Ed25519 key (both loaded once at startup, read-only).
- All instances must use the same Ed25519 signing key (shared via env var or
  mounted volume).
- Rate limiting moves to Redis or a shared store when running multiple instances.
- Idempotency key checks use database unique constraints, so they work across instances.

---

## 11. Non-Goals

- **No marketplace.** Skills are registered per-tenant via API. No discovery, search, or cross-tenant sharing.
- **No billing.** Usage counters exist but no metering, invoicing, or payment integration.
- **No complex org/teams model.** One tenant = one API key = one skill namespace. No roles, no RBAC beyond key-level access.
- **No remote policy editing UI.** Tenants use the bundled `ops-v0.1` profile. Custom bundles require backend configuration.
- **No raw artifact parsing in core.** Evidra server never parses terraform plans, k8s manifests, or cloud API responses. Input adapters (§4b) exist as a separate module for CI integrations. AI agents need no adapters — the LLM is the parser.
- **No agent framework lock-in.** The API is HTTP + JSON. No SDK required. No MCP-specific protocol in the HTTP layer.
- **No skill versioning.** Skills can be updated in place. No version history or rollback in MVP.
- **No async execution.** All policy evaluation is synchronous.
- **No server-side evidence storage.** Evidence is returned to the client. The server is stateless with respect to audit data.

**Scaling triggers (post-MVP):**
- \>50 active keys → add org/team support
- \>10k requests/day → add async usage counter writes (batch insert)
- p95 latency >200ms → profile OPA evaluation, add engine caching
- \>100 keys/week issuance → add email verification or abuse detection

### Known Tradeoffs

Accepted limitations that do not need fixing now but should be understood.

**`input_hash` instability across server versions.** The hash is computed via
`json.Marshal` over Go structs — deterministic within a single binary but not
across Go versions or struct changes. During a rolling update of a multi-node
cluster, two nodes running different versions may produce different hashes for
identical input. This does not affect correctness (each evidence record is
self-contained) but breaks cross-node dedup by `input_hash`. If multi-node
dedup becomes a requirement, replace with a hash over a deterministic canonical
form (similar to `signing_payload`).

**`usage_counters` upsert contention.** The `ON CONFLICT ... DO UPDATE` upsert
on `(tenant_id, endpoint, bucket)` serializes concurrent writes to the same
row. At sustained >1k req/s from a single tenant to a single endpoint, this
becomes a lock contention hotspot. For MVP traffic this is irrelevant. First
mitigation: buffer increments in memory and flush in batch every 1-5 seconds.
Second: move to a time-series store (TimescaleDB, ClickHouse).

**Idempotent replay does not return `evidence_record`.** If the client failed
to persist the evidence from the original response, it is lost — the server
does not store it and the replay returns only metadata. This is a deliberate
cost/complexity tradeoff: storing evidence server-side defeats the stateless
architecture. For SaaS UX, a future improvement would be a client-side SDK
that persists evidence before returning the response to the caller, making loss
a client bug rather than an architectural gap.

**`unknown_key_id` verify reason is forward-looking.** MVP uses a single
signing key, so `unknown_key_id` can never occur today. The reason code exists
for key rotation (post-MVP): when multiple keys are in play, a record signed
by a retired key that has been removed from `/v1/evidence/pubkey` would trigger
this. Keeping the code path now avoids a protocol-breaking change later.

---

## 12. Implementation Plan

### Launch Phases

**Phase 1 = public launch.** Phase 2 = hidden behind `EVIDRA_SKILLS_ENABLED`.
DB migration creates all tables upfront — enabling Phase 2 requires zero schema changes.

### Phase 1 Task Sequence (Public Launch)

**1A: Signing infrastructure**

| # | Task | DoD |
|---|---|---|
| 1.1 | `internal/evidence/types.go` — `EvidenceRecord`, `DecisionRecord` structs | Structs defined with `SigningPayload` and `Signature` fields. JSON tags match spec. No `omitempty` on fields included in signing payload. |
| 1.2 | `internal/evidence/signer.go` — Ed25519 `Signer` | `BuildSigningPayload` (length-prefixed list encoding), `Sign`, `Verify`, `lengthPrefixedJoin`, `parseLengthPrefixed`. Key loading from env var or file — never writes to disk. Fail-fast when no key and `EVIDRA_ENV != development`. Ephemeral in-memory key in dev only. Unit tests: sign/verify round-trip, length-prefixed encode/decode with commas and special chars, cross-verification. |
| 1.3 | `internal/evidence/builder.go` — `BuildRecord` | Builds `EvidenceRecord` from `engine.Result` + `invocation.ToolInvocation`. Unit tests. |

**1B: Database and storage**

| # | Task | DoD |
|---|---|---|
| 1.4 | Write `001_initial.up.sql` migration | All tables: tenants, api_keys, usage_counters, skills, executions. All created upfront (Phase 2 tables exist but are unused until flag is on). `down.sql` reverses cleanly. |
| 1.5 | `internal/storage/postgres.go` | Connection pool init, ping. |
| 1.6 | `internal/storage/tenants.go` — `TenantRepo` | `Create`. Unit tests with test DB. |
| 1.7 | `internal/storage/apikeys.go` — `APIKeyRepo` | `Create`, `FindByHash`, `Revoke`, `TouchLastUsed`. Unit tests. |
| 1.8 | `internal/storage/usage.go` — `UsageRepo` | `Increment` (upsert). Unit tests. |

**1C: Auth and engine**

| # | Task | DoD |
|---|---|---|
| 1.9 | `internal/auth/` | Dual-mode auth: Phase 0 (constant-time compare against `EVIDRA_API_KEY`, synthetic tenant) and Phase 1+ (SHA-256 hash, DB lookup). Key generation, parsing, Bearer extraction, tenant context. Unit tests for both modes. |
| 1.10 | `internal/engine/adapter.go` | Init `runtime.Evaluator` once, expose `Evaluate` method. Unit tests. |

**1D: HTTP handlers (Phase 1 endpoints)**

| # | Task | DoD |
|---|---|---|
| 1.11 | `internal/api/keys_handler.go` | `POST /v1/keys`. Handler tests. |
| 1.12 | `internal/api/validate_handler.go` | `POST /v1/validate`. Returns signed evidence record with `signing_payload`. Handler tests. |
| 1.13 | `internal/api/verify_handler.go` | `POST /v1/evidence/verify` (opt-in via `EVIDRA_VERIFY_ENABLED`, requires auth), `GET /v1/evidence/pubkey` (public). Verify checks signature over `signing_payload` and confirms payload matches structured fields. Handler tests. |
| 1.14 | `internal/api/health_handler.go` | `GET /healthz`, `GET /readyz`. |
| 1.15 | Wire router + middleware | Mount Phase 1 handlers. Pubkey handler without auth. Verify handler behind auth + `EVIDRA_VERIFY_ENABLED` flag. Rate limiter, logging, recovery, body limit. Feature flag middleware for `/v1/skills/*` and `/v1/executions/*` → `404`. |

**1E: Landing page**

| # | Task | DoD |
|---|---|---|
| 1.16 | Static HTML landing page | Single page with "Get API Key" form. `embed` in Go binary or served by reverse proxy. Calls `POST /v1/keys`, displays key once. |

**1F: Integration tests**

| # | Task | DoD |
|---|---|---|
| 1.17 | **Phase 0: stateless flow** | No `DATABASE_URL`. Start with `EVIDRA_API_KEY=test-key`. `POST /v1/validate` with Bearer test-key → signed evidence_record. `GET /v1/evidence/pubkey` → public key. Offline verify succeeds. `POST /v1/keys` → `404`. `GET /readyz` → `200`. |
| 1.18 | Phase 1: validate flow | Issue key via `POST /v1/keys` → `POST /v1/validate` → verify signed evidence_record → verify signature via pubkey endpoint. |
| 1.19 | Deny flow | Validate with kube-system target → verify denied (HTTP 200, `allow: false`) → verify evidence_record still signed and returned. |
| 1.20 | Evidence verify (opt-in) | With `EVIDRA_VERIFY_ENABLED=true`: take evidence_record → `POST /v1/evidence/verify` **with API key** → valid. Tamper field → `payload_field_mismatch`. Tamper signature → `signature_mismatch`. Without flag: → `404`. Without API key: → `401`. |
| 1.21 | Cross-language signing payload | Reconstruct `signing_payload` from evidence_record JSON fields in test (simulating a non-Go client). Must match the `signing_payload` in the response exactly. Test must include records with non-empty `reasons`/`hints` containing commas and special characters to verify length-prefixed encoding. |
| 1.22 | Feature flags off | `EVIDRA_SKILLS_ENABLED=false`: skills endpoints → `404`. No `DATABASE_URL`: keys endpoint → `404`. |
| 1.23 | Static key in Phase 1+ | Set both `DATABASE_URL` and `EVIDRA_API_KEY`. Static key authenticates without DB lookup. Dynamic-issued key also works. Both produce valid evidence. |

**1G: Docker + smoke test**

| # | Task | DoD |
|---|---|---|
| 1.24 | `docker-compose.yml` | `evidra-api` + `postgres:16`. Health check passes. |
| 1.25 | Phase 0 smoke test | Start binary without Postgres. `EVIDRA_API_KEY=test`. Validate → evidence → offline verify. |
| 1.26 | Phase 1 smoke test | Issue key → validate (allow) → validate (deny) → verify evidence → verify pubkey. All pass. |

### Phase 2 Task Sequence (Skills — Feature Flag)

**2A: Skills storage and validation**

| # | Task | DoD |
|---|---|---|
| 2.1 | `internal/storage/skills.go` — `SkillRepo` | `Create`, `FindByID`, `FindByName`, `ListByTenant`, `Update`, `SoftDelete`. Unit tests. |
| 2.2 | `internal/storage/executions.go` — `ExecutionRepo` | `Create`, `FindByID`, `FindByIdempotencyKey`. Unit tests. |
| 2.3 | JSON Schema validation library | `github.com/santhosh-tekuri/jsonschema/v5` or equivalent. |
| 2.4 | `internal/skills/validator.go` | `ValidateSchema(schema)`, `ValidateInput(schema, input)`. Unit tests. |
| 2.5 | `internal/skills/builder.go` | `BuildInvocation(skill, input, actor, env) → ToolInvocation`. Target merging, risk tag injection. Unit tests. |

**2B: HTTP handlers (Phase 2 endpoints)**

| # | Task | DoD |
|---|---|---|
| 2.6 | `internal/api/skills_handler.go` | `POST/GET /v1/skills`, `GET/PUT/DELETE /v1/skills/{id}`. Handler tests. |
| 2.7 | `internal/api/execute_handler.go` | `POST /v1/skills/{id}:simulate`, `POST /v1/skills/{id}:execute`, `GET /v1/executions/{id}`. Idempotent replay returns minimal response. Handler tests. |
| 2.8 | Mount behind feature flag | Skills handlers registered in router but gated by `EVIDRA_SKILLS_ENABLED`. |

**2C: Integration tests (Phase 2)**

| # | Task | DoD |
|---|---|---|
| 2.9 | Skill CRUD | Register → list → get → update → soft delete. Against real Postgres. |
| 2.10 | Execute flow | Register skill → execute (allow) → verify execution record → verify evidence_record signature. |
| 2.11 | Simulate flow | Register skill → simulate → no evidence_record → no execution record. |
| 2.12 | Idempotency | Execute with key → replay → same execution_id, `idempotent_replay: true`, no evidence_record. |
| 2.13 | Tenant isolation | Tenant A registers skill → Tenant B cannot see/execute. |
| 2.14 | Smoke test update | Add: register skill → simulate → execute → get execution to existing smoke script. |

### Test Plan

| Layer | Tool | Phase 1 | Phase 2 |
|---|---|---|---|
| Unit | `go test` | Signer, builder, auth, key/tenant/usage repos, validate handler | Skills repo, executions repo, schema validator, invocation builder, skills/execute handlers |
| Integration | `go test` + test DB | Validate → sign → verify flow, deny flow, cross-language payload | Skill CRUD, execute, simulate, idempotency, tenant isolation |
| Docker smoke | Shell script | Key → validate → verify | + skill → simulate → execute |
| Race | `go test -race` | All packages | All packages |

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
- [ ] Public endpoints (`/v1/keys`, `/v1/evidence/pubkey`) have no auth middleware
- [ ] Opt-in endpoints (`/v1/evidence/verify`) gated by feature flag and auth