# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog.

## [v0.2.0] — 2026-03-10

Project renamed to **Evidra-Lock**. All binaries, Docker images, repo URLs, and documentation updated.

### Added

- **Engine v2** — rewritten evaluation pipeline with Rego canonicalization layer.
  - `canonicalize.rego` projects raw action payloads into normalized flat shape before rule evaluation.
  - `input.actions` list replaces single-action input — supports multi-action validation.
  - Actor-aware decisions: `actor.type` (`human|agent|ci`) drives rule behavior (e.g. `ops.autonomous_execution` warn).
  - Tool field semantics documented: `tool` = execution CLI, not manifest generator. `oc` treated as `kubectl`.

- **Deny-loop prevention** (`stop_after_deny`) — opt-in cache that blocks agents from retrying identical denied operations. Enable with `--deny-cache` or `EVIDRA_DENY_CACHE=true`.

- **OpenShift support** — `oc` recognized as equivalent to `kubectl` for canonicalization and sufficient-context rules.

- **MCP input schema** — `validate` tool ships a JSON Schema describing expected input structure, improving agent payload accuracy.

- **Agent contract prompts** — externalized MCP system prompts and tool descriptions (`prompts/mcpserver/`). Agent contract v1 defines validate-before-execute protocol.

- **Deny hints** — all deny decisions return actionable hints (1–3 per rule) from `rule_hints/data.json`.

- **MCP Registry publications** — published to both Docker MCP Registry (`mcp/evidra`) and modelcontextprotocol.io (`io.github.vitas/evidra-lock`). Docker Desktop zero-config install.

- **Hosted endpoints** — `https://evidra-lock.samebits.com/mcp` (MCP) and `https://api.evidra.rest/v1` (REST API).

- **E2E test suite** — data-driven tests using Claude Code headless against real MCP server. Corpus-backed with `tests/corpus/`. Supports Sonnet (blocking) and Haiku (signal) model runs. HTML report generation.

- **UI improvements** — landing page rewrite with kill-switch positioning, interactive prompt table, hosted MCP one-liner setup, MCP client configs for Claude Code / Desktop / Cursor / Codex / Gemini CLI / OpenClaw.

- **Policy additions** — `ops.unapproved_change` (protected namespace gate), `ops.public_exposure` (Terraform public exposure), `ops.autonomous_execution` (audit warn), `ops.breakglass_used` (audit warn). Total: 23 rules.

- **Claude Code Skill** (`skills/evidra-infra-safety/`) — installable skill for automatic validate-before-mutate behavior.

- **Homebrew tap** — `brew install samebits/tap/evidra-lock-mcp`.

### Changed

- **Renamed to Evidra-Lock** — repo URLs (`github.com/vitas/evidra-lock`), binary names (`evidra-lock`, `evidra-lock-mcp`, `evidra-lock-api`), Docker images (`ghcr.io/vitas/evidra-lock-*`), all documentation and UI.
- Engine evaluates `defaults.actions` (canonicalized) instead of raw `input.params` — rules no longer depend on agent payload structure.
- Embedded bundle cache moved to `~/.evidra/bundles/ops-v0.1/` (survives restarts).
- MCP server prompts and tool descriptions externalized from Go code to `prompts/mcpserver/`.
- Policy bundle restructured: insufficient context rule split into per-tool sufficient-context clauses.
- UI redesigned with terminal demo, scenario grid, and docs section with API quickstart.

### Documentation

- Engine Logic v2 spec (`docs/ENGINE_LOGIC_V2.md`).
- Model Behavior and Determinism guide (`docs/MODEL_BEHAVIOR_AND_DETERMINISM.md`).
- MCP setup guide with configs for 6 agent platforms (`docs/mcp-setup.md`).
- MCP Registry publication guide (`docs/docker-mcp-registry.md`).
- Development guide with UI mock mode (`docs/evidra-development-guide.md`).
- Roadmap v0.3.0–v0.5.0+ (`ROADMAP.md`).

---

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

- **Policy bundle expanded** from the initial baseline to a curated ops rules layer.
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
