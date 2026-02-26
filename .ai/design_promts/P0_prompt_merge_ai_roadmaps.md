# Prompt: Merge & Update .ai Roadmap Files

**Target:** Replace both files with a single consolidated file:
- DELETE `.ai/AI_CLAUDE_P0_MCP_FIRST_ROADMAP.md`
- DELETE `.ai/AI_CLAUDE_PRODUCT_ROADMAP_00.md`
- CREATE `.ai/ROADMAP.md` — single source of truth for implementation roadmap

**Context:** These two files were written 2026-02-25 when the project was CLI + MCP only. Since then:
1. Most of original P0 was implemented (bundle embedding, zero-config, install paths, README)
2. We designed a full hosted API, hybrid mode, adapter system, deployment infrastructure
3. Policy library was expanded to 23 rules
4. The project direction shifted from "MCP-first local tool" to "API-first with offline fallback"

The old files have significant overlap (P0 MCP-FIRST is a subset of PRODUCT_ROADMAP_00). They reference items that are done, and don't mention the API/hybrid/adapter work at all.

---

## Status of items from old roadmaps

### From P0 (MCP-FIRST) — status:

| # | Item | Status |
|---|---|---|
| 1 | Embed bundle — zero-config `evidra-mcp` startup | ✅ DONE |
| 2 | Install path: Homebrew + Docker | ✅ DONE (goreleaser, GHCR) |
| 3 | MCP-first README and demo | ✅ DONE |
| 4 | 3-minute MCP quickstart | ✅ DONE |

### From P0.1 (Engineering Polish) — status:

| Item | Status |
|---|---|
| Release pipeline hardening | ✅ DONE (goreleaser snapshot, smoke tests) |
| Binary size verification | ✅ DONE |
| CI hardening | ✅ DONE (bundle-test, race detector) |
| Trust signals | ✅ DONE (badges, LICENSE, SECURITY.md) |

### From P1 — status:

| # | Item | Status |
|---|---|---|
| 0 | Serious Baseline Policy Pack (23 rules) | ✅ DONE |
| 1 | HTTP transport on MCP server | 🔄 SUPERSEDED by Hosted API design |
| 2 | Structured logging (slog) | ⬜ NOT STARTED (still relevant) |
| 3 | `list_rules` + `simulate` MCP tools | ⬜ NOT STARTED |
| 4 | GitHub Action scaffold | ⬜ NOT STARTED (designed, not implemented) |

### From Quick Wins Sprint — status:

| Days | Item | Status |
|---|---|---|
| 1-2 | Zero-config binary | ✅ DONE |
| 3-4 | README rewrite + demo GIF | ✅ DONE |
| 5 | Structured logging | ⬜ NOT STARTED |
| 6-7 | `list_rules` + `simulate` MCP tools | ⬜ NOT STARTED |
| 8 | GoReleaser Docker + Homebrew | ✅ DONE |
| 9-12 | Policy Pack (17 new rules) | ✅ DONE |
| 13 | `evidra evidence report` | ⬜ NOT STARTED |
| 14 | HTTP transport | 🔄 SUPERSEDED by API |
| 15 | GitHub Action scaffold | ⬜ NOT STARTED |
| 16 | Release v0.1.0 | ✅ DONE |

### NEW work designed but not in old roadmaps:

| Item | Status | Design Doc |
|---|---|---|
| Hosted API (Phase 0, stateless) | ⬜ DESIGNED | `docs/architecture_hosted-api-mvp.md` |
| Hosted API (Phase 1, PostgreSQL) | ⬜ DESIGNED | same, §12 Phase 1 tasks |
| Hosted API (Phase 2, Skills) | ⬜ DESIGNED | same, §12 Phase 2 tasks |
| Hybrid mode (CLI + MCP → API-first) | ⬜ DESIGNED | `evidra_hybrid_mode_design.md` |
| `pkg/client` — HTTP client | ⬜ DESIGNED | same |
| `pkg/mode` — mode resolution | ⬜ DESIGNED | same |
| Input adapter interface | ⬜ DESIGNED | `evidra_adapter_system_design.md` |
| `evidra-adapter-terraform` v0.1.0 | ⬜ DESIGNED | same |
| `evidra/adapters` repo setup | ⬜ DESIGNED | `prompt_adapters_release_infra.md` |
| `evidra-infra` repo (Hetzner deploy) | ⬜ DESIGNED | `evidra_deployment_hetzner.md` |
| Dogfooding CI (infra validates itself) | ⬜ DESIGNED | `prompt_dogfooding_ci.md` |
| UI/Landing page | ⬜ DESIGNED | `evidra_ui_system_design.md` |
| Architecture docs (public + private) | ⬜ DESIGNED | `evidra_architecture_public.html` |

---

## New Roadmap Structure

The new `.ai/ROADMAP.md` should have:

### Header

```markdown
# Evidra — Implementation Roadmap

**Updated:** 2026-02-26
**Status:** P0-local complete. P0-api in design. Implementation not started.
```

### Section 1: What's Shipped (v0.1.x)

Brief summary of everything that works today. Reference items from old P0/P0.1/P1 that are done. This is the "ground truth" — no aspirational language.

Key points:
- CLI (`evidra validate`) — offline OPA evaluation
- MCP server (`evidra-mcp`) — validate tool + get_event resource
- Policy engine: OPA with ops-v0.1 bundle (23 rules, embedded)
- Evidence: hash-linked JSONL, append-only, local storage
- Install: Homebrew, Docker (GHCR), goreleaser
- README: demo GIF, quickstart, copy-paste MCP config
- CI: bundle-test, race detector, badges

### Section 2: P0-API — Current Focus (~2-3 weeks implementation)

This is the next body of work. All design is complete. Implementation not started.

Organize by implementation order (dependencies):

**Step 1: Create `evidra-infra` repo**
- Terraform IaC for Hetzner
- docker-compose (Traefik v3 + evidra-api + PostgreSQL)
- Deploy scripts
- Design: `evidra_deployment_hetzner.md`

**Step 2: Implement API Phase 0 (stateless)**
- `cmd/evidra-api` — HTTP server
- `internal/evidence/signer.go` — Ed25519 signing
- `internal/auth/` — static API key (Phase 0)
- `internal/engine/` — wrap existing OPA engine
- Endpoints: `POST /v1/validate`, `GET /v1/evidence/pubkey`, `GET /healthz`
- Design: `docs/architecture_hosted-api-mvp.md` §12, tasks 1.1-1.14 (Phase 0 subset)

**Step 3: Deploy to Hetzner**
- CX22 (€4.67/mo), Traefik v3, Let's Encrypt
- Domain: `evidra.rest`
- GitHub Actions: build → push GHCR → deploy

**Step 4: Implement hybrid mode**
- `pkg/client` — HTTP client for API
- `pkg/mode` — mode resolution (online/offline)
- CLI updates: `--url`, `--api-key`, `--offline`, `--fallback-offline`
- MCP updates: `EVIDRA_URL` support, conditional bundle extraction
- Design: `evidra_hybrid_mode_design.md` (7 steps)

**Step 5: Create `evidra/adapters` repo**
- Adapter interface + `evidra-adapter-terraform`
- goreleaser, Dockerfile, Makefile, tests
- Design: `prompt_adapters_release_infra.md`

**Step 6: Dogfooding CI**
- `evidra-infra` PRs run terraform plan → adapter → POST /v1/validate
- Design: `prompt_dogfooding_ci.md`

**P0-API Exit Criteria:**
- `curl -X POST https://evidra.rest/v1/validate -H "Authorization: Bearer ..." → signed evidence`
- `evidra validate --url https://evidra.rest scenario.yaml` works
- `evidra-mcp` with `EVIDRA_URL=https://evidra.rest` delegates to API
- `terraform show -json | evidra-adapter-terraform | evidra validate -` works end-to-end
- Infra PRs validated by Evidra (dogfooding)

**P0-API is the stopping point.** Everything below is designed but not scheduled.

### Section 3: Backlog — Designed, Not Scheduled

List these with one-line descriptions and design doc references. No timelines.

**API Phase 1 (multi-tenant):**
- PostgreSQL, dynamic key issuance, usage tracking, landing page
- Design: `docs/architecture_hosted-api-mvp.md` §12 Phase 1 tasks

**API Phase 2 (skills):**
- Skills API, MCP dynamic tools, execute/simulate
- Design: same, §12 Phase 2 tasks

**Structured logging (slog):**
- `log/slog` in `pkg/mcpserver` and `pkg/validate`
- From old P1 item #2. Still relevant, low effort.

**`list_rules` + `simulate` MCP tools:**
- From old P1 item. Useful but not blocking.

**GitHub Action:**
- `evidra/action@v1` — designed in `evidra_adapter_system_design.md`
- Depends on API + adapter being live

**`evidra evidence report`:**
- From old quick wins. Nice-to-have.

**Evidence sync (local → API):**
- Requires API evidence ingestion endpoint. Phase 2+.

### Section 4: Strategic Context (condensed from old files)

Keep the good parts from the old executive summary:
- "Core is sound, periphery was missing" → periphery is now mostly built
- MCP is still the differentiator
- Evidence chain is still underexploited in messaging
- Keep the "avoid these traps" list

Remove:
- Anything about "six rules" (now 23)
- "Stdio-only is a constraint" (API solves this)
- "Zero discoverability" (Homebrew/Docker/GHCR exist now)
- "3-week sprint" plan (done)
- The entire goreleaser yaml, Dockerfile content, workflow details (these live in actual repos now)
- "Brutal truth" closing (outdated)

### Section 5: Design Documents Index

Table of all design docs produced, with one-line descriptions:

| Document | Location | Scope |
|---|---|---|
| API MVP Architecture | `docs/architecture_hosted-api-mvp.md` | Full API design (2200 lines) |
| Hybrid Mode Design | (external) `evidra_hybrid_mode_design.md` | CLI/MCP hybrid mode implementation |
| Adapter System Design | (external) `evidra_adapter_system_design.md` | Input adapter interface + terraform adapter |
| UI System Design | (external) `evidra_ui_system_design.md` | Landing page, console, dashboard |
| Deployment Runbook | (external) `evidra_deployment_hetzner.md` | Hetzner infrastructure |
| Skills Integration | (external) `evidra_skills_integration_strategy.md` | Where skills plug in |
| Repo Organization | (external) `evidra_repo_organization.md` | Multi-module monorepo design |

Note: "(external)" means the doc was produced in design sessions but needs to be committed to the repo.

---

## Writing Guidelines

- Plain markdown, no emoji headers
- Remove all "Generated on:" timestamps
- Remove complexity/impact/owner columns (this is a solo project)
- Don't reproduce goreleaser yaml, Dockerfiles, workflow content — those live in actual files
- Keep it under 200 lines total
- Tone: direct inventory of what's done and what's next
- No marketing language, no "brutal truth" editorializing
