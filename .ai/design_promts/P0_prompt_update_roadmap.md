# Prompt: Update Evidra Roadmap & Product Direction

**Target files:**
- `docs/EVIDRA_STRATEGIC_VISION_ROADMAP.md` — replace entirely
- `docs/EVIDRA_V1_PRODUCT_DIRECTION.md` — replace entirely

**Context:** These files were written on 2026-02-22 when Evidra was a local CLI + MCP tool. Since then, we've designed a full hosted API with hybrid mode. The old Phase 1/2/3 framing ("Sharp Validator → Policy Intelligence → Trust & Attestation") is obsolete — we've already designed Ed25519 signing, input adapters, and a hosted API. The new roadmap should reflect what actually exists and what comes next.

---

## What exists today (v0.1 — shipped)

- CLI (`evidra validate`) — offline OPA evaluation against scenario files
- MCP server (`evidra-mcp`) — exposes `validate` tool + `get_event` resource for AI agents
- Policy engine: OPA with `ops-v0.1` bundle (23 rules, embedded)
- Evidence: hash-linked JSONL chain, append-only, local storage
- Scenario format: action lists, Terraform plans, K8s manifests

## What we've designed (v0.2 — in progress)

All design docs are complete. Implementation has NOT started.

### Hosted API (Phase 0 — stateless, no DB)
- `POST /v1/validate` — same policy engine, server-side Ed25519 signing
- `GET /v1/evidence/pubkey` — public key for offline verification
- `GET /healthz`
- Static API key from env var
- Single binary, zero external deps except OPA
- Design: `docs/architecture_hosted-api-mvp.md` (§1-§7, §10, §12 Phase 0 tasks)

### Hosted API (Phase 1 — with PostgreSQL)
- `POST /v1/keys` — dynamic key issuance + tenant creation
- Usage tracking, rate limiting
- Landing page
- Design: same doc, §12 Phase 1 tasks

### Hosted API (Phase 2 — Skills, feature-flagged)
- `POST /v1/skills`, `POST /v1/skills/{id}:execute`
- Named operations with input schema + policy enforcement
- Design: same doc, §12 Phase 2 tasks

### Hybrid Mode (v0.2.0)
- CLI and MCP become API-first when `EVIDRA_URL` is set
- Offline fallback with `EVIDRA_FALLBACK=offline`
- Fail closed by default
- New packages: `pkg/client`, `pkg/mode`
- Design: `evidra_hybrid_mode_design.md` (7 implementation steps)

### Three-repo architecture
- `evidra/evidra` (public) — API server, MCP server, CLI, OPA engine
- `evidra/adapters` (public) — adapter interface + terraform plan adapter
- `evidra-infra` (private) — Terraform IaC, docker-compose, deploy scripts
- Zero Go coupling between repos. HTTP JSON only.

### Input Adapter System
- `evidra-adapter-terraform` — stdin pipe, converts tfplan.json → ToolInvocation
- Adapter interface: `Name()`, `Convert()`, filter semantics
- Separate Go module, goreleaser cross-compilation
- Design: `evidra_adapter_system_design.md`

### Deployment
- Hetzner CX22 (€4.67/mo), Traefik v3, PostgreSQL, Let's Encrypt
- Domain: `evidra.rest`
- Design: `evidra_deployment_hetzner.md`, `evidra_deployment_runbook_private.html`

### Dogfooding CI
- `evidra-infra` PRs run terraform plan → adapter → POST /v1/validate
- "Evidra validates Evidra" story

---

## New Roadmap Structure

Replace the old 3-phase structure with this:

### P0 — API + Hybrid (current focus, ~2-3 weeks)

**Goal:** Working hosted API at `evidra.rest`, CLI/MCP talk to it or work offline.

What's included:
- Hosted API Phase 0 (stateless, Ed25519 signing, static API key)
- Hybrid mode in CLI + MCP (online/offline/fallback)
- `pkg/client` + `pkg/mode` packages
- Input adapter: `evidra-adapter-terraform` v0.1.0
- Hetzner deployment (Traefik + single binary)
- Dogfooding CI for infra repo
- Updated CLAUDE.md, architecture docs

**Stopping point:** P0 is where we stop for now. Everything below is "designed but not scheduled."

Exit criteria:
- `curl POST https://evidra.rest/v1/validate` returns signed evidence
- `evidra validate --url https://evidra.rest` works
- `evidra-mcp` with `EVIDRA_URL` delegates to API
- `evidra-adapter-terraform | evidra validate -` works
- Infra PRs validated by Evidra

### P1 — Multi-tenant + Landing (future, unscheduled)

What it adds:
- PostgreSQL-backed key management (`POST /v1/keys`)
- Tenant isolation, usage tracking
- Landing page with "Get API Key" form
- Rate limiting per key/IP

Gate: `DATABASE_URL` set → enables Phase 1 features

### P2 — Skills + Integrations (future, unscheduled)

What it adds:
- Skills API (`POST /v1/skills`, `:execute`, `:simulate`)
- MCP dynamic tools (one tool per registered skill)
- GitHub Action (`evidra/action@v1`)
- Terraform Cloud run task integration

Gate: `EVIDRA_SKILLS_ENABLED=true`

### P3 — Scale + Enterprise (future, unscheduled)

What it adds:
- Evidence sync (local → API)
- Kubernetes-native deployment
- SSO / OIDC
- Audit dashboard

---

## Product Direction Update

Replace the old "v1 Product Direction" doc with a shorter, updated version:

### Positioning (updated)
> Policy guardrails for AI infrastructure agents — with signed evidence.

### What changed from v1 vision:
- Ed25519 signing is NOT Phase 3 anymore — it's P0 (already designed)
- Hosted API is NOT "optional expansion" — it's the primary interface
- CLI is now an API client with offline fallback, not the primary tool
- Adapters are external (separate repo/binary), not built into core
- MCP is API-first (delegates to server), with local OPA as fallback

### Product filter (same as before, still valid):
> Does it strengthen deterministic validation of infrastructure outcomes?
> If no → reject or postpone.

### Release Readiness (P0):
- [ ] API Phase 0 deployed and accessible
- [ ] Hybrid mode in CLI + MCP
- [ ] Terraform adapter v0.1.0 released
- [ ] Architecture docs up to date
- [ ] README updated to show both online and offline workflows
- [ ] Dogfooding CI validates infra PRs

---

## Writing Guidelines

- Remove ALL emoji headers (🎯 🧠 etc.) — they're noise
- Use plain markdown with `##` headers
- Remove generated-on timestamps
- Keep the "avoid these traps" section — it's still relevant
- Tone: direct, technical, no marketing language
- Length: roadmap ~100 lines, product direction ~60 lines
