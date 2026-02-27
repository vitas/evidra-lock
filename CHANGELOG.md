# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog.

## [v0.1.0] — Phase 1

### Added

- **`POST /v1/keys`** — dynamic API key issuance backed by Postgres. Returns plaintext key once with `Cache-Control: no-store`. Rate-limited: 3 keys/hour/IP. Optional `EVIDRA_INVITE_SECRET` invite gate.
- **`GET /readyz`** — readiness probe that verifies database connectivity (registered only when `DATABASE_URL` is set).
- **DB-backed auth** (`internal/auth`) — `KeyStoreMiddleware` performs SHA-256 hash lookup with per-key tenant isolation and async `last_used_at` tracking.
- **`internal/store/`** — `KeyStore`: `CreateKey`, `LookupKey` (primitive returns, satisfies `auth.KeyLookup`), `TouchKey` (async). pgx/v5, raw SQL, no ORM.
- **`internal/db/`** — `Connect()`: pgxpool init + ping + embedded migration runner. Idempotent DDL — safe to re-run on restart. No external migration framework.
- **Migrations** — `001_keys.sql`: `tenants` and `api_keys` tables with `key_hash BYTEA` (SHA-256), unique index on hash, index on tenant.
- **Phase auto-detect** — server reads `DATABASE_URL` at startup; Phase 0 path (static key auth, no `/readyz`) unchanged when absent.

---

## [v0.0.5] — Phase 0 complete

### Added

- **API server** (`cmd/evidra-api`) — stateless HTTP server for policy evaluation.
  - `POST /v1/validate` — evaluate policy, return Ed25519-signed evidence record.
  - `GET /v1/evidence/pubkey` — Ed25519 public key (PEM) for offline verification.
  - `GET /healthz` — liveness probe.
  - Static API key auth with constant-time compare and 50-100ms timing jitter on failure.
  - Body limit middleware (1MB).
  - Deny = HTTP 200 (policy denial is a successful evaluation, not an error).

- **Ed25519 evidence signing** (`internal/evidence/`).
  - Deterministic text signing payload (`evidra.v1` format, fixed field order).
  - Length-prefixed encoding for list fields.
  - Key loading from base64 env var, PEM file, or ephemeral dev mode.
  - Sign/verify round-trip with `crypto/ed25519` stdlib.

- **Hybrid mode** — CLI and MCP become API-first when `EVIDRA_URL` is set.
  - `pkg/client` — HTTP client for `POST /v1/validate` with sentinel errors (`ErrUnreachable`, `ErrUnauthorized`, `ErrServerError`, etc.) and `IsReachabilityError()` for fallback classification.
  - `pkg/mode` — mode resolution (online/offline), pure config validation, no I/O.
  - CLI flags: `--url`, `--api-key`, `--offline`, `--fallback-offline`, `--timeout`.
  - MCP: `EVIDRA_URL` support, conditional bundle extraction, cached bundles at `~/.evidra/bundles/ops-v0.1/`.
  - Fallback policy: `EVIDRA_FALLBACK=offline` falls back to local OPA on API failure; `closed` (default) errors immediately.
  - CLI exit codes: 0=allowed, 1=internal error, 2=denied, 3=API unreachable, 4=usage error.

- **Policy bundle expanded** from 6 to 23 rules.
  - Kubernetes: `k8s.privileged_container`, `k8s.host_namespace_escape`, `k8s.run_as_root`, `k8s.hostpath_mount`, `k8s.dangerous_capabilities`, `k8s.mutable_image_tag`, `k8s.no_resource_limits` (CIS 5.2.x, kube-score).
  - Terraform: `terraform.sg_open_world`, `terraform.s3_public_access`, `terraform.iam_wildcard_policy` (tfsec AVD-AWS).
  - AWS S3: `aws_s3.no_encryption`, `aws_s3.no_versioning_prod`.
  - AWS IAM: `aws_iam.wildcard_policy`, `aws_iam.wildcard_principal`.
  - ArgoCD: `argocd.autosync_prod`, `argocd.wildcard_destination`, `argocd.dangerous_sync_combo`.

- **`NormalizeEnvironment()`** in `pkg/config` — normalizes `prod`/`prd` → `production`, `stg`/`stage` → `staging`.

- **UI** — landing page scaffold (`ui/`).

- **Install paths** — Homebrew tap, Docker images on GHCR, goreleaser cross-compilation.
- **CI** — bundle-test workflow, race detector, API server CI, `verify_p0.sh`.
- **Demo GIF** in README.

### Changed

- MCP server caches embedded bundle to `~/.evidra/bundles/ops-v0.1/` instead of tmpdir (survives restarts).
- Evidence records include `source` field (`api`, `local`, `local-fallback`).
- Renamed AWS policy rules with `aws_` prefix for consistency.

### Documentation

- Architecture doc rewrite — system overview, component map, hybrid mode, API surface, evidence signing, deployment, security model.
- Security model doc — API auth, signing key management, input validation, logging/redaction.
- Roadmap consolidation — merged P0 MCP-first and product roadmap into single `.ai/ROADMAP.md`.
- Product direction update — reflects API-first shift.
- Policy catalog, contributing guide, docs index.

## [v0.0.1]

- MCP server for AI agents with `validate` and `get_event` tools.
- OPA/Rego policy enforcement with structured decisions (`allow`, `risk_level`, `reason`).
- `ops-v0.1` policy bundle with 6 initial rules (Kubernetes, Terraform, ops).
- Segmented append-only evidence store with sealing and hash-chain validation.
- Evidence utilities: `verify`, `violations`, `export`.
- `evidra version` command with build metadata.
- GitHub templates and CI workflows.
- GoReleaser configuration and release workflow.
