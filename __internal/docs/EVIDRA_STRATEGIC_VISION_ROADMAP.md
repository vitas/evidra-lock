# Evidra — Strategic Vision and Roadmap

**Updated:** 2026-02-26

---

## Current State (v0.1 — shipped)

- CLI (`evidra validate`) — OPA evaluation against scenario files, online and offline modes
- MCP server (`evidra-mcp`) — `validate` tool + `get_event` resource for AI agents, enforce/observe modes
- Policy engine: OPA with `ops-v0.1` bundle (23 rules, embedded), environment-aware `by_env` parameters
- Evidence: hash-linked JSONL chain, append-only, local storage
- Install: Homebrew, Docker (GHCR), goreleaser cross-compilation
- Docs: architecture, security model, policy catalog, quickstart

---

## P0 — API + Hybrid (current focus)

**Goal:** Working hosted API at `evidra.rest`. CLI and MCP talk to it or work offline.

### API Phase 0 (stateless, no database)

- `POST /v1/validate` — policy evaluation with Ed25519-signed evidence
- `GET /v1/evidence/pubkey` — public key for offline verification
- `GET /healthz` — liveness probe
- Static API key from env var, constant-time compare, timing jitter on auth failure
- Single binary, zero dependencies beyond OPA
- Design: `__internal/docs/implemented/evidra_sysdesign-api-mvp.md`

### Hybrid mode

- CLI and MCP become API-first when `EVIDRA_URL` is set
- Local OPA fallback with `EVIDRA_FALLBACK=offline`
- Fail closed by default — API unreachable + `fallback=closed` = error
- New packages: `pkg/client` (HTTP client), `pkg/mode` (mode resolution)
- Design: `__internal/docs/implemented/evidra_cli_hybrid_mode_design.md`

### Deployment

- Hetzner CX22, Traefik v3, Let's Encrypt TLS
- Domain: `evidra.rest`
- GitHub Actions: build → push GHCR → deploy

### Input adapters

- `evidra/adapters` — separate Go module, zero import coupling with main repo
- `evidra-adapter-terraform` — reads `terraform show -json`, produces structured ToolInvocation params
- Design: `__internal/docs/implemented/evidra_adapter_system_design.md`

### Dogfooding CI

- Infrastructure PRs: `terraform plan` → adapter → `POST /v1/validate`

### Exit criteria

- `curl POST https://evidra.rest/v1/validate` returns signed evidence
- `evidra validate --url https://evidra.rest` works
- `evidra-mcp` with `EVIDRA_URL` delegates to API
- `evidra-adapter-terraform | evidra validate -` works end-to-end
- Infrastructure PRs validated by Evidra

**P0 is the stopping point.** Everything below is designed but not scheduled.

---

## P1 — Multi-tenant + Landing (unscheduled)

Gate: `DATABASE_URL` set → enables Phase 1 features.

- PostgreSQL-backed key management (`POST /v1/keys`, `GET /readyz`)
- Tenant isolation, usage tracking, rate limiting per key/IP
- Landing page with "Get API Key" form
- Design: `__internal/docs/implemented/evidra_sysdesign-api-mvp.md` Phase 1 tasks

---

## P2 — Skills + Integrations (unscheduled)

Gate: `EVIDRA_SKILLS_ENABLED=true`

- Skills API: `POST /v1/skills`, `POST /v1/skills/{id}:execute`, `:simulate`
- MCP dynamic tools (one tool per registered skill)
- GitHub Action (`evidra/action@v1`)
- Design: same doc, Phase 2 tasks

---

## P3 — Scale + Enterprise (unscheduled)

- Evidence sync (local → API)
- Kubernetes-native deployment
- SSO / OIDC
- Audit dashboard

---

## Avoid These Traps

- Becoming a generic DevSecOps platform
- Building compliance-first messaging before the API is proven
- Creating approval workflow engines
- Expanding into policy-as-a-service complexity
- Supporting too many execution modes simultaneously
- Adding an ORM or web framework — stdlib `net/http` + `database/sql` + `pgx`
- Building a dashboard before the API is live and dogfooded

Complexity is the main long-term risk.

---

## Positioning

> Policy guardrails for AI infrastructure agents — with signed evidence.

Product filter (unchanged):

> Does it strengthen deterministic validation of infrastructure outcomes? If no — reject or postpone.
