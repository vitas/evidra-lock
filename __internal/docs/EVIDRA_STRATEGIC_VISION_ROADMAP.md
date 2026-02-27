# Evidra — Strategic Vision and Roadmap

**Updated:** 2026-02-27

---

## Current State (v0.1 — shipped)

- CLI (`evidra validate`) — OPA evaluation against scenario files, online and offline modes
- MCP server (`evidra-mcp`) — `validate` tool + `get_event` resource for AI agents, enforce/observe modes
- Policy engine: OPA with `ops-v0.1` bundle (23 rules, embedded), environment-aware `by_env` parameters
- Evidence: hash-linked JSONL chain, append-only, local storage
- Install: Homebrew, Docker (GHCR), goreleaser cross-compilation
- Docs: architecture, security model, policy catalog, quickstart

---

## ✅ P0 — API + Hybrid (complete)

**Shipped.** Working hosted API at `api.evidra.rest`. CLI and MCP talk to it or work offline.

- API Phase 0 (stateless): `POST /v1/validate`, `GET /v1/evidence/pubkey`, `GET /healthz`. Ed25519-signed evidence, static key auth with timing jitter.
- Hybrid mode: CLI and MCP become API-first when `EVIDRA_URL` is set. `pkg/client` + `pkg/mode`. Configurable fallback.
- Deployment: Hetzner CX22, Traefik v3, Let's Encrypt TLS. GitHub Actions CI/CD.
- Input adapters: `evidra/adapters` repo, `evidra-adapter-terraform`.
- Dogfooding CI: infra PRs validated by Evidra.

---

## ✅ P1 — Multi-tenant + Key Issuance (complete)

Gate: `DATABASE_URL` set → Phase 1 features auto-enabled.

- `POST /v1/keys` — dynamic key issuance. `ev1_` + 32 random bytes base62, SHA-256 hash stored. Rate limited: 3/hr/IP. Optional invite gate (`EVIDRA_INVITE_SECRET`). Returns key once with `Cache-Control: no-store`.
- `GET /readyz` — readiness probe with DB ping.
- `internal/store/` — `CreateKey`, `LookupKey` (primitive returns, satisfies `auth.KeyLookup`), `TouchKey` (async).
- `internal/db/` — `Connect()`: pgxpool + embedded migration runner, idempotent DDL.
- Auth auto-switch: `KeyStoreMiddleware` when store present, `StaticKeyMiddleware` (P0) otherwise.

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
