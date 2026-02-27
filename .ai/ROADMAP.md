# Evidra — Implementation Roadmap

**Updated:** 2026-02-26
**Status:** P0-local complete. P0-API designed, implementation not started.

---

## 1. What's Shipped (v0.1.x)

Everything below works today and is released.

**CLI** (`evidra validate`) — OPA evaluation against scenario files (Terraform plan JSON, Kubernetes manifests, native format). Supports offline (local OPA) and online (`EVIDRA_URL`) modes with configurable fallback. Policy simulation via `evidra policy sim`. Evidence inspection via `evidra evidence verify|export|violations`.

**MCP server** (`evidra-mcp`) — stdio transport for AI agents. Two tools: `validate` (policy evaluation + evidence) and `get_event` (evidence lookup). Three resources for evidence inspection. Enforce and observe modes. Zero-config startup with embedded `ops-v0.1` bundle.

**Policy engine** — OPA with `ops-v0.1` bundle (23 rules). Covers Kubernetes container escape (CIS 5.2.x), Terraform public exposure, IAM wildcard policies, S3 encryption/versioning, ArgoCD operational safety. Environment-aware parameters via `by_env` model.

**Evidence** — hash-linked JSONL, append-only, segmented storage at `~/.evidra/evidence`. Chain verification via `evidra evidence verify`.

**Install** — Homebrew (`brew install samebits/evidra/evidra`), Docker (GHCR), goreleaser for cross-platform binaries.

**CI** — `bundle-test.yml` (OPA tests on policy changes), race detector in test matrix, badges (CI, Go Report Card), `LICENSE`, `SECURITY.md`.

**Docs** — README with demo GIF, copy-paste MCP config, quickstart. Architecture, security model, policy catalog, contributing guide.

---

## 2. P0-API — Current Focus

All design is complete. Implementation not started. Steps are ordered by dependency.

### Step 1: Implement API Phase 0 (stateless)

- `cmd/evidra-api` — HTTP server, stdlib `net/http`
- `internal/evidence/` — Ed25519 signer, deterministic signing payload, evidence record builder
- `internal/auth/` — static API key with constant-time compare, timing jitter
- `internal/engine/` — thin wrapper around existing `pkg/runtime.Evaluator`
- Endpoints: `POST /v1/validate`, `GET /v1/evidence/pubkey`, `GET /healthz`
- Deny = HTTP 200. Evidence signed and returned, never stored server-side.
- Design: `__internal/docs/implemented/evidra_sysdesign-api-mvp.md`

### Step 2: Deploy to Hetzner

- Terraform IaC for Hetzner CX22 (2 vCPU, 4 GB)
- docker-compose: Traefik v3 (Let's Encrypt TLS) + evidra-api
- Domain: `evidra.rest`
- GitHub Actions: build, push GHCR, deploy

### Step 3: Implement hybrid mode

- `pkg/client` — HTTP client for `POST /v1/validate` with sentinel errors and fallback classification
- `pkg/mode` — mode resolution (online/offline), pure config validation, no I/O
- CLI updates: `--url`, `--api-key`, `--offline`, `--fallback-offline` flags
- MCP updates: `EVIDRA_URL` support, conditional bundle extraction, fallback to local eval
- Design: `__internal/docs/implemented/evidra_cli_hybrid_mode_design.md`

### Step 4: Create adapters repo

- `evidra/adapters` — separate Go module, zero import coupling
- Adapter interface: `Name() string`, `Convert(ctx, raw, config) → Result`
- `evidra-adapter-terraform` — reads `terraform show -json`, produces structured params
- goreleaser, Dockerfile, stdin/stdout binary distribution
- Design: `__internal/docs/implemented/evidra_adapter_system_design.md`

### Step 5: Dogfooding CI

- Infrastructure PRs run: `terraform plan` → adapter → `POST /v1/validate`
- Evidra validates its own infrastructure changes

### P0-API Exit Criteria

- `curl -X POST https://api.evidra.rest/v1/validate -H "Authorization: Bearer ..." → signed evidence`
- `evidra validate --url https://api.evidra.rest scenario.yaml` works
- `evidra-mcp` with `EVIDRA_URL=https://api.evidra.rest` delegates to API
- `terraform show -json | evidra-adapter-terraform | evidra validate -` works end-to-end
- Infrastructure PRs validated by Evidra

---

## 3. Backlog — Designed, Not Scheduled

### API Phase 1 (multi-tenant)

PostgreSQL, dynamic key issuance (256-bit keys, SHA-256 hashed), usage tracking, `POST /v1/keys`, `GET /readyz`. Design: `__internal/docs/implemented/evidra_sysdesign-api-mvp.md` Phase 1 tasks.

### API Phase 2 (skills)

Skills API: `POST /v1/skills`, `POST /v1/skills/{id}:execute`, `POST /v1/skills/{id}:simulate`. Named operations with JSON Schema input validation. Design: same, Phase 2 tasks.

### Structured logging (slog)

`log/slog` in `pkg/mcpserver` and `pkg/validate`. `--log-level` and `--log-format` flags. Structured fields per evaluation: event_id, tool, operation, allow, risk_level, duration_ms. Low effort, no new dependencies.

### `list_rules` + `simulate` MCP tools

`list_rules`: build rule index at startup from bundle hints/params, expose as MCP tool. `simulate`: same as validate but `skip_evidence: true` — policy dry-run without recording.

### GitHub Action

`evidra/action@v1` — inputs: `input-file`, `environment`, `fail-on-deny`. Downloads binary from GitHub Releases for runner OS/arch. Depends on API + adapter being live. Design: `__internal/docs/implemented/evidra_adapter_system_design.md`.

### `evidra evidence report`

`evidra evidence report --format markdown|text` — event count, risk distribution, top denied rule IDs, chain status.

### UI / Landing page

Landing page, API console, dashboard. Design: `__internal/docs/implemented/evidra_ui_system_design.md`.

### Evidence sync (local to API)

Upload local evidence to API for centralized storage. Requires API evidence ingestion endpoint. Phase 2+.

---

## 4. Strategic Context

**Core is sound, periphery is now mostly built.** The evaluation pipeline (scenario → OPA → evidence) was correct from day one. What was missing — installability, policy depth, documentation, install paths — has been addressed. The next gap is the hosted API and hybrid mode.

**MCP is still the differentiator.** No other open-source tool positions itself as a safety layer between an AI agent and infrastructure execution. The stdio MCP integration works today; the API extends this to remote agents and CI pipelines.

**Evidence chain is underexploited in messaging.** Hash-linked append-only evidence (offline) and Ed25519-signed records (online) are strong trust primitives. They should be more prominent in positioning.

**Avoid these traps:**
- Do not add an ORM or web framework. stdlib `net/http` + `database/sql` + `pgx`.
- Do not build a dashboard before the API is live and dogfooded.
- Do not add MCP tools (`list_rules`, `simulate`) before hybrid mode works — agent developers need online mode first.
- Do not optimize for enterprise features (RBAC, multi-tenant) before single-tenant is proven.

---

## 5. Design Documents Index

| Document | Location | Scope |
|---|---|---|
| API MVP Architecture | `__internal/docs/implemented/evidra_sysdesign-api-mvp.md` | Full API design: phases, endpoints, signing, auth |
| Hybrid Mode Design | `__internal/docs/implemented/evidra_hybrid_mode_design.md` | CLI/MCP API-first with offline fallback |
| CLI Hybrid Mode Spec | `__internal/docs/implemented/evidra_cli_hybrid_mode_design.md` | Implementation spec for CLI hybrid mode |
| Adapter System Design | `__internal/docs/implemented/evidra_adapter_system_design.md` | Input adapter interface + terraform adapter |
| UI System Design | `__internal/docs/implemented/evidra_ui_system_design.md` | Landing page, console, dashboard |
| Repo Organization | `__internal/docs/backlog/evidra_repo_organization.md` | Multi-module monorepo plan |
| Skills Integration | `__internal/docs/ideas/evidra_skills_integration_strategy.md` | Where skills plug in |
| Architecture (public) | `docs/ARCHITECTURE.md` | System overview, component map, API surface |
| Security Model (public) | `docs/SECURITY_MODEL.md` | Enforcement, evidence integrity, key management |
| Policy Catalog (public) | `docs/POLICY_CATALOG.md` | Full rule reference (23 rules) |
