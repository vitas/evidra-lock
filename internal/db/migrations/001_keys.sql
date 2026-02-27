-- Phase 1: tenant and API key tables.
-- All statements are idempotent (IF NOT EXISTS) — safe to re-run on restart.

CREATE TABLE IF NOT EXISTS tenants (
    id         TEXT        PRIMARY KEY,          -- ULID
    label      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id           TEXT        PRIMARY KEY,         -- ULID
    tenant_id    TEXT        NOT NULL REFERENCES tenants(id),
    key_hash     BYTEA       NOT NULL,            -- SHA-256(plaintext), 32 bytes
    prefix       TEXT        NOT NULL,            -- "ev1_<first8>" for log correlation
    label        TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,                     -- NULL = active
    last_used_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_hash   ON api_keys (key_hash);
CREATE        INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys (tenant_id);
