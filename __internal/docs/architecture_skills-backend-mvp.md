# Skills Backend MVP — Architecture

**Date:** 2026-02-25
**Status:** Draft
**Base layer:** [Hosted API MVP](./architecture_hosted-api-mvp.md)
**Scope:** Extend the hosted API with skill registration, pre-execution guardrails, and execution tracking

---

## 1. Overview

### What "Skills Backend" Means

A **skill** is a named, schema-validated AI agent action that is subject to Evidra policy before execution. The skills backend provides a registration and execution API so that platforms (CI systems, agent orchestrators, internal tools) can:

1. Register skill definitions with input schemas and policy metadata.
2. Request execution of a skill — triggering Evidra policy evaluation **before** any infrastructure action runs.
3. Track execution lifecycle and retrieve linked evidence records.

The skills backend does **not** execute infrastructure operations itself. It evaluates policy, records the decision, and returns the verdict. The calling platform is responsible for acting on the decision.

### Relationship to Hosted API MVP

The skills backend is an extension layer. It reuses the following components from the hosted API MVP without modification:

| Component | Reused from Hosted API MVP |
|---|---|
| Authentication | Bearer key auth, `api_keys` table, SHA-256 hashing, auth middleware |
| Tenant isolation | `tenant_id` from key, `WHERE tenant_id = $1` on all queries |
| Policy engine | `internal/engine.Adapter` → `pkg/runtime.Evaluator` → OPA bundle |
| Evidence persistence | `evidence` table (Postgres JSONB), hash-linked chain per tenant |
| Error model | `{"ok": false, "error": {"code": "...", "message": "..."}}` |
| Rate limiting | In-memory token bucket per key/IP |
| Observability | Structured `slog` logging, Prometheus metrics, `usage_counters` |
| Deployment | Single Go binary, Postgres, reverse proxy, `golang-migrate` |

New components added by the skills backend: `skills` table, `executions` table, `execution_evidence` join table, skills HTTP handlers, execution state machine.

---

## 2. Use Cases (MVP)

### UC-1: Platform Registers a Skill

A platform operator registers a skill definition via API. The definition includes a name, input JSON schema, and policy metadata (risk tags, environment scope). Once registered, agents can reference the skill by ID.

### UC-2: Agent Executes a Skill

An agent (or the platform on behalf of an agent) calls the execute endpoint with a skill ID and input payload. The skills backend:
1. Validates input against the skill's schema.
2. Builds a `ToolInvocation` from the skill definition + input.
3. Evaluates policy via the existing Evidra engine.
4. Records evidence.
5. Returns the decision and an execution ID.

If policy denies, the response includes rule IDs, reasons, and hints. The platform decides whether to abort or surface the denial to the agent.

### UC-3: Policy Blocks Execution Before Infrastructure Call

The execute endpoint evaluates policy **synchronously** before returning. A deny decision is returned with `"allow": false` and full diagnostic data. No execution record is created with status `completed` — it stays `denied`. The platform never reaches the infrastructure call.

### UC-4: Audit Trail Linked to Execution

Every execution produces an evidence record. The `execution_evidence` table links `execution_id` to one or more `event_id` values. Auditors can query by execution ID to find all policy decisions made for that execution, or query by evidence ID to find the execution context.

---

## 3. Non-Goals

- **No marketplace.** Skills are registered per-tenant via API. No discovery, search, or cross-tenant sharing.
- **No billing.** Usage counters exist (from hosted API MVP) but no metering, invoicing, or payment integration.
- **No complex org/teams model.** One tenant = one API key = one skill namespace. No roles, no RBAC beyond key-level access.
- **No remote policy editing UI.** Tenants use the bundled `ops-v0.1` profile. Custom bundles require backend configuration.
- **No agent framework lock-in.** The API is HTTP + JSON. No SDK required. No MCP-specific protocol in the HTTP layer (MCP users continue using the MCP server directly).
- **No skill versioning.** Skills can be updated in place. No version history or rollback in MVP.
- **No async execution.** All policy evaluation is synchronous. The execute endpoint returns the decision inline.

---

## 4. Architecture

### High-Level Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        Reverse Proxy (TLS)                               │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                         evidra-api (Go)                                   │
│                                                                          │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐  ┌───────────────┐   │
│  │ Router   │→ │ Auth MW  │→ │ Handlers           │→ │ Engine        │   │
│  │ (stdlib) │  │ (Bearer) │  │                    │  │ Adapter       │   │
│  └──────────┘  └──────────┘  │ keys_handler       │  └───────┬───────┘   │
│                              │ validate_handler    │          │           │
│                              │ evidence_handler    │    ┌─────▼────────┐  │
│                              │ skills_handler  ◀──NEW  │ pkg/validate │  │
│                              │ execute_handler ◀──NEW  │ pkg/runtime  │  │
│                              │ health_handler      │   │ pkg/policy   │  │
│                              └─────────┬──────────┘   └──────────────┘  │
│                                        │                                 │
│                                  ┌─────▼──────────┐                     │
│                                  │ Storage         │                     │
│                                  │ (Postgres)      │                     │
│                                  │                 │                     │
│                                  │ tenants         │ ← existing          │
│                                  │ api_keys        │ ← existing          │
│                                  │ evidence        │ ← existing          │
│                                  │ usage_counters  │ ← existing          │
│                                  │ skills      ◀──NEW                   │
│                                  │ executions  ◀──NEW                   │
│                                  │ execution_evidence ◀──NEW            │
│                                  └────────┬────────┘                    │
│                                           │                              │
└───────────────────────────────────────────┼──────────────────────────────┘
                                            │
                                            ▼
                                 ┌──────────────────┐
                                 │   PostgreSQL      │
                                 └──────────────────┘
```

### Data Flow: Execute Endpoint

```
POST /v1/skills/{skill_id}:execute
  │
  ├─ 1. Auth middleware → tenant_id
  ├─ 2. Load skill definition (skills table, WHERE tenant_id AND skill_id)
  ├─ 3. Validate input against skill.input_schema (JSON Schema draft-07)
  ├─ 4. Build ToolInvocation from skill template + input
  │       actor   ← from request body or skill defaults
  │       tool    ← skill.tool
  │       operation ← skill.operation
  │       params.target  ← skill.default_target merged with input
  │       params.payload ← input payload
  │       params.risk_tags ← skill.risk_tags
  │       context.source ← "skills-api"
  │       context.intent ← input.intent (optional)
  │       environment ← input.environment || skill.default_environment
  │
  ├─ 5. Engine.Evaluate(ToolInvocation) → Decision
  ├─ 6. INSERT evidence record (existing evidence table)
  ├─ 7. INSERT execution record (new executions table)
  ├─ 8. INSERT execution_evidence link
  ├─ 9. Increment usage_counters
  └─ 10. Return response: decision + execution_id + event_id
```

---

## 5. API Design

All endpoints require `Authorization: Bearer ev1_...` unless noted. All responses use the hosted API error model: `{"ok": false, "error": {"code": "...", "message": "..."}}`.

Content-Type: `application/json` for all requests and responses.

---

### `POST /v1/skills`

Register a new skill definition.

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

List all skills for the authenticated tenant.

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

Retrieve a single skill definition including the full `input_schema`.

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

Dry-run policy evaluation. No evidence is written. No execution record is created.

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

Execute policy evaluation, record evidence, create execution tracking record.

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
  "event_id": "evt-1740477600000000000",
  "decision": {
    "allow": true,
    "risk_level": "low",
    "reason": "all_policies_passed",
    "reasons": [],
    "rule_ids": [],
    "hints": []
  },
  "status": "allowed",
  "skill_id": "sk_01JEXAMPLE",
  "created_at": "2026-02-25T10:00:00Z"
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
  "created_at": "2026-02-25T10:00:00Z"
}
```

Note: a denied execution returns `200` with `"ok": true` and `"status": "denied"`. The HTTP status reflects that the request was processed correctly. The `decision.allow` field carries the policy verdict.

**Idempotency:** If `idempotency_key` matches an existing execution for this tenant, the original response is returned without re-evaluation. Idempotency keys expire after 24 hours.

**Errors:**
- `400` — input fails skill schema validation, or missing required fields
- `401` — missing or invalid key
- `404` — skill not found
- `422` — policy evaluation error
- `429` — rate limit
- `500` — evidence write failure

---

### `GET /v1/executions/{execution_id}`

Retrieve an execution record with linked evidence IDs.

**Response (200):**
```json
{
  "ok": true,
  "execution": {
    "id": "ex_01JEXAMPLE",
    "skill_id": "sk_01JEXAMPLE",
    "skill_name": "deploy-service",
    "status": "allowed",
    "decision": {
      "allow": true,
      "risk_level": "low",
      "reason": "all_policies_passed",
      "reasons": [],
      "rule_ids": [],
      "hints": []
    },
    "input_hash": "sha256:abc123...",
    "environment": "prod",
    "actor": {
      "type": "agent",
      "id": "deploy-bot",
      "origin": "api"
    },
    "event_ids": ["evt-1740477600000000000"],
    "idempotency_key": "deploy-api-v2.3-20260225T100000",
    "created_at": "2026-02-25T10:00:00Z"
  }
}
```

**Errors:**
- `401` — missing or invalid key
- `404` — execution not found (or belongs to another tenant)

---

### Rate Limits (Skills Endpoints)

| Endpoint | Limit |
|---|---|
| `POST /v1/skills` | 30 req/min per key |
| `GET /v1/skills`, `GET /v1/skills/{id}` | 120 req/min per key |
| `POST /v1/skills/{id}:simulate` | 60 req/min per key (same as validate) |
| `POST /v1/skills/{id}:execute` | 60 req/min per key (same as validate) |
| `GET /v1/executions/{id}` | 120 req/min per key |

---

## 6. Data Model

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

```sql
CREATE TABLE executions (
    id                TEXT        PRIMARY KEY,              -- "ex_" + ULID
    tenant_id         TEXT        NOT NULL REFERENCES tenants(id),
    skill_id          TEXT        NOT NULL REFERENCES skills(id),
    status            TEXT        NOT NULL,                  -- "allowed" | "denied" | "error"
    decision_allow    BOOLEAN     NOT NULL,
    decision_risk     TEXT        NOT NULL,                  -- "low" | "medium" | "high"
    decision_reason   TEXT        NOT NULL DEFAULT '',
    rule_ids          TEXT[]      NOT NULL DEFAULT '{}',
    environment       TEXT        NOT NULL DEFAULT '',
    actor_type        TEXT        NOT NULL,
    actor_id          TEXT        NOT NULL,
    actor_origin      TEXT        NOT NULL,
    input_hash        TEXT        NOT NULL,                  -- SHA-256 of validated input
    idempotency_key   TEXT,                                  -- optional, unique per tenant within TTL
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_executions_tenant_created ON executions (tenant_id, created_at DESC);
CREATE INDEX idx_executions_tenant_skill ON executions (tenant_id, skill_id, created_at DESC);
CREATE UNIQUE INDEX idx_executions_idempotency
    ON executions (tenant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL
    AND created_at > now() - INTERVAL '24 hours';
```

**Notes:**
- `status` is terminal at creation time. No status transitions in MVP (no async execution).
- `idempotency_key` uniqueness is enforced within a 24-hour window. A daily cleanup job removes the partial index filter by deleting rows where `idempotency_key IS NOT NULL AND created_at < now() - INTERVAL '7 days'` (index covers 24h; cleanup runs at 7d for safety).

---

### `execution_evidence`

Links executions to evidence records. In MVP, each execution produces exactly one evidence record. The join table exists to support future scenarios where a single execution involves multiple policy evaluations (e.g., multi-action skills).

```sql
CREATE TABLE execution_evidence (
    execution_id    TEXT        NOT NULL REFERENCES executions(id),
    evidence_id     TEXT        NOT NULL REFERENCES evidence(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (execution_id, evidence_id)
);

CREATE INDEX idx_exec_evidence_evidence ON execution_evidence (evidence_id);
```

---

### `api_keys`

No changes. Defined in [Hosted API MVP §3](architecture_hosted-api-mvp.md).

---

### Retention

| Table | Retention |
|---|---|
| `skills` | Indefinite (soft-deleted rows cleaned after 90 days) |
| `executions` | 90 days (daily cleanup of old rows) |
| `execution_evidence` | Cascades with `executions` cleanup |
| `evidence` | Indefinite (audit trail) |
| `usage_counters` | 90 days (from hosted API MVP) |

---

### Migration

```
internal/migrate/migrations/
  001_initial.up.sql        ← existing (tenants, api_keys, evidence, usage_counters)
  002_skills.up.sql         ← NEW (skills, executions, execution_evidence)
  002_skills.down.sql       ← NEW
```

---

## 7. Skill Definition Schema

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

## 8. Security Model

### Authentication

Same as hosted API MVP. Bearer key with `ev1_` prefix. Key lookup via SHA-256 hash index. See [Hosted API MVP §4](architecture_hosted-api-mvp.md).

### Key Rotation

MVP supports key revocation (`api_keys.revoked_at`). Rotation workflow:
1. Issue new key (`POST /v1/keys`).
2. Update platform configuration to use new key.
3. Revoke old key (admin operation — no self-service API in MVP).

The new key inherits the same `tenant_id`, so all skills and executions are accessible.

### Threat Model

| Threat | Mitigation |
|---|---|
| **Input injection via skill input** | Input validated against registered `input_schema` before `ToolInvocation` construction. Schema is set at registration time, not by the caller. |
| **Risk tag escalation** | `risk_tags` are fixed in the skill definition. Execute request cannot override them. |
| **Replay attacks** | `idempotency_key` deduplication. Evidence hash chain detects replayed records. TLS required for all external traffic. |
| **Privilege escalation via skill mutation** | Skills can only be mutated by the same tenant's API key. No cross-tenant access. Skill updates do not retroactively change existing execution records. |
| **SSRF** | The skills backend does not make outbound HTTP calls. It only evaluates policy locally. Skill definitions contain metadata, not URLs to fetch. |
| **Evidence tampering** | Hash-linked chain per tenant (from hosted API MVP). Tampering is detectable via chain validation. |
| **Enumeration** | 404 returned for all not-found cases regardless of existence. Skill IDs use ULIDs (not sequential). |

### Logging Redaction

Same rules as hosted API MVP:
- **Never log:** plaintext API keys, input payloads (may contain infrastructure secrets).
- **Always log:** key prefix, tenant_id, skill_id, execution_id, event_id, decision (allow/deny/risk_level), latency.
- Input payloads are persisted as `input_hash` (SHA-256) in the execution record. The full input is stored in the evidence table's `params_jsonb` column for audit replay, encrypted at rest via Postgres TDE or disk-level encryption.

### Rate Limiting and Abuse Controls

- Per-key rate limits defined in §5.
- Max 200 skills per tenant.
- Max `input_schema` size: 64 KB.
- Max request body: 1 MB (from hosted API MVP).
- Skill name regex enforced at registration.

---

## 9. Execution Semantics

### Simulate vs Execute

| | Simulate | Execute |
|---|---|---|
| Endpoint | `POST /v1/skills/{id}:simulate` | `POST /v1/skills/{id}:execute` |
| Input validation | Yes (against `input_schema`) | Yes (against `input_schema`) |
| Policy evaluation | Yes | Yes |
| Evidence written | **No** | **Yes** |
| Execution record | **No** | **Yes** |
| Usage counter | Incremented (endpoint: `simulate`) | Incremented (endpoint: `execute`) |
| Idempotency key | Not supported | Supported |

`simulate` is for dry-run / pre-flight checks. Agents can call simulate to preview a decision before committing to execute.

### When Evidence Is Written

Evidence is written **only** on `execute`, **regardless** of whether the decision is allow or deny. This ensures the audit trail captures all execution attempts, including blocked ones.

Evidence is written **after** policy evaluation and **before** the response is returned. If the evidence write fails, the endpoint returns `500` and no execution record is created. This matches the existing Evidra guarantee: no silent evidence gaps.

### Decision and Execution State

| Decision | Execution Status | Meaning |
|---|---|---|
| `allow: true` | `"allowed"` | Policy passed. Platform should proceed with the infrastructure action. |
| `allow: false` | `"denied"` | Policy blocked. Platform should not proceed. |
| (evaluation error) | `"error"` | Policy engine failed. Platform should treat as deny. |

Execution status is terminal at creation. There are no status transitions in MVP.

### Idempotency

- The `idempotency_key` field is optional on execute requests.
- If provided, the backend checks for an existing execution with the same `(tenant_id, idempotency_key)` created within the last 24 hours.
- If found, the original response is returned without re-evaluation.
- If not found, a new execution proceeds normally.
- Keys are strings, max 128 chars, set by the caller. Recommended format: `{skill-name}-{unique-context}-{timestamp}`.

### Timeouts

- Policy evaluation timeout: 5 seconds (OPA engine). If exceeded, returns `422` with `"code": "policy_timeout"`.
- Request timeout: 30 seconds (HTTP server). Covers full request lifecycle.
- Database query timeout: 10 seconds.

### Retries

The skills backend does not retry internally. If a transient failure occurs (DB unavailable, lock contention), the endpoint returns an error and the caller retries. Idempotency keys make retries safe for execute.

---

## 10. Observability

### Structured Log Fields

Every request to skills/execute endpoints logs these fields via `slog`:

```json
{
  "level": "info",
  "msg": "skill_execute",
  "request_id": "req-abc123",
  "tenant_id": "01JEXAMPLE",
  "key_prefix": "ev1_a8Fk3mQ",
  "skill_id": "sk_01JEXAMPLE",
  "skill_name": "deploy-service",
  "execution_id": "ex_01JEXAMPLE",
  "event_id": "evt-1740477600000000000",
  "decision_allow": true,
  "decision_risk": "low",
  "status": "allowed",
  "environment": "prod",
  "duration_ms": 12,
  "method": "POST",
  "path": "/v1/skills/sk_01JEXAMPLE:execute",
  "http_status": 200
}
```

Log levels:
- `info` — request completed (no payload content logged).
- `warn` — rate limit hit, idempotency key collision, input validation failure.
- `error` — evidence write failure, policy engine error, DB error.

### Prometheus Metrics

Additional metrics beyond hosted API MVP:

```
evidra_skills_total{tenant_id}                                       gauge
evidra_skill_executions_total{skill_id, status, risk_level}          counter
evidra_skill_execution_duration_seconds{skill_id}                     histogram
evidra_skill_simulations_total{skill_id, allow}                       counter
evidra_skill_input_validation_failures_total{skill_id}                counter
```

### Audit Logging

- `execute` calls produce evidence records (stored in `evidence` table). This is the primary audit mechanism.
- `simulate` calls are counted but do not produce evidence records.
- No payload content at `info` log level. Full payloads are only in evidence records (DB, not logs).

---

## 11. Deployment

### Binary

Same binary as the hosted API MVP. Skills handlers are registered alongside existing handlers.

```bash
go build -o bin/evidra-api ./cmd/evidra-api
```

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

### Environment Variables

New variables (in addition to hosted API MVP variables):

| Variable | Required | Default | Description |
|---|---|---|---|
| `EVIDRA_SKILLS_MAX_PER_TENANT` | no | `200` | Maximum skills per tenant |
| `EVIDRA_SKILLS_INPUT_SCHEMA_MAX_BYTES` | no | `65536` | Maximum input_schema size |
| `EVIDRA_EXECUTE_TIMEOUT_SEC` | no | `5` | Policy evaluation timeout |

All existing variables from [Hosted API MVP §7](architecture_hosted-api-mvp.md) apply unchanged.

### Reverse Proxy

Same as hosted API MVP. TLS termination at reverse proxy (nginx/Caddy/cloud LB). Go binary listens on plain HTTP.

### Local Dev Mode

```bash
# Start Postgres
docker run -d --name evidra-pg -e POSTGRES_DB=evidra -e POSTGRES_PASSWORD=dev -p 5432:5432 postgres:16

# Run migrations + start server
DATABASE_URL="postgres://postgres:dev@localhost:5432/evidra?sslmode=disable" \
  bin/evidra-api --migrate
```

### Scaling Notes (Post-MVP)

- The single-binary architecture supports horizontal scaling behind a load balancer.
- All state is in Postgres. No in-process state beyond the OPA engine (loaded once at startup, read-only).
- Rate limiting moves to Redis or a shared store when running multiple instances.
- Idempotency key checks use database unique constraints, so they work across instances.

---

## 12. Implementation Plan

### Task Sequence

**Phase 1: Data layer (skills + executions)**

| # | Task | DoD |
|---|---|---|
| 1.1 | Write `002_skills.up.sql` migration | Tables created, indexes verified, `down.sql` reverses cleanly |
| 1.2 | `internal/storage/skills.go` — `SkillRepo` | `Create`, `FindByID`, `FindByName`, `ListByTenant`, `Update`, `SoftDelete`. Unit tests with test DB. |
| 1.3 | `internal/storage/executions.go` — `ExecutionRepo` | `Create`, `FindByID`, `ListBySkill`. Idempotency key lookup. Unit tests. |
| 1.4 | `internal/storage/execution_evidence.go` — `ExecutionEvidenceRepo` | `Link`, `FindByExecution`, `FindByEvidence`. Unit tests. |

**Phase 2: Skill input validation**

| # | Task | DoD |
|---|---|---|
| 2.1 | Add JSON Schema validation library | `github.com/santhosh-tekuri/jsonschema/v5` or equivalent. Validate schemas at registration, validate inputs at execute/simulate. |
| 2.2 | `internal/skills/validator.go` | `ValidateSchema(schema)` — checks schema is valid JSON Schema draft-07. `ValidateInput(schema, input)` — validates input against schema. Unit tests with edge cases. |

**Phase 3: ToolInvocation builder**

| # | Task | DoD |
|---|---|---|
| 3.1 | `internal/skills/builder.go` | `BuildInvocation(skill, input, actor, env) → ToolInvocation`. Handles target merging, risk tag injection, context construction. Unit tests. |

**Phase 4: HTTP handlers**

| # | Task | DoD |
|---|---|---|
| 4.1 | `internal/api/skills_handler.go` | `POST /v1/skills`, `GET /v1/skills`, `GET /v1/skills/{skill_id}`. Handler tests with mock storage. |
| 4.2 | `internal/api/execute_handler.go` | `POST /v1/skills/{id}:simulate`, `POST /v1/skills/{id}:execute`, `GET /v1/executions/{id}`. Handler tests with mock storage + mock engine. |
| 4.3 | Wire into router | Mount new handlers in `router.go`. Verify auth middleware applies. |

**Phase 5: Integration tests**

| # | Task | DoD |
|---|---|---|
| 5.1 | Skill CRUD integration test | Register → list → get → update → soft delete. Against real Postgres. |
| 5.2 | Execute flow integration test | Register skill → execute (allow) → verify execution record → verify evidence record → verify execution_evidence link. |
| 5.3 | Deny flow integration test | Register skill with target kube-system → execute → verify denied → verify evidence still recorded. |
| 5.4 | Simulate flow integration test | Register skill → simulate → verify no evidence → verify no execution record. |
| 5.5 | Idempotency test | Execute with key → re-execute with same key → verify same execution_id returned → verify single evidence record. |
| 5.6 | Tenant isolation test | Tenant A registers skill → Tenant B cannot see/execute it. |

**Phase 6: Docker smoke test**

| # | Task | DoD |
|---|---|---|
| 6.1 | `docker-compose.yml` | `evidra-api` + `postgres:16`. Health check passes. |
| 6.2 | Smoke test script | Issue key → register skill → simulate → execute → get execution → get evidence. All pass. |

### Test Plan

| Layer | Tool | Coverage target |
|---|---|---|
| Unit | `go test` | Storage repos, schema validator, invocation builder, handler logic |
| Integration | `go test` with test DB | Full request flow through HTTP → storage → engine → evidence |
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