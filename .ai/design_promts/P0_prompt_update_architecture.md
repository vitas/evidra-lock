# Prompt: Update docs/architecture.md

**Target file:** `docs/architecture.md`

**Context:** The current `docs/architecture.md` is 19 lines — a single Mermaid diagram and one-paragraph descriptions of 6 modules. It was written for v0.1 (CLI + MCP only). We've since designed a full hosted API, hybrid mode, input adapters, three-repo architecture, and Hetzner deployment. This file needs to become the authoritative architecture reference for the project.

**Important:** This is NOT `docs/architecture_hosted-api-mvp.md` (which is the 2200-line API-specific design doc). This is `docs/architecture.md` — the project-level overview that should give someone the complete picture in ~200-300 lines.

---

## What the file should contain

### 1. System Overview (~30 lines)

One-paragraph description of what Evidra is. Then a Mermaid diagram showing the full system:

```
Components to show:
- AI Agent (Claude Code, Cursor, etc.)
- evidra-mcp (stdio transport)
- evidra CLI
- evidra-api (HTTP server)
- OPA policy engine (embedded in all three)
- PostgreSQL (optional, Phase 1+)
- Evidence (client-side JSONL + server-signed)
- Input Adapters (external, separate repo)
- CI Pipeline (GitHub Actions)
```

Show two paths:
1. **Online:** Agent → MCP → API → OPA → signed evidence returned
2. **Offline:** Agent → MCP → local OPA → local evidence

### 2. Three-Repo Architecture (~20 lines)

```
evidra/evidra (public)     — API server, MCP, CLI, OPA engine, evidence
evidra/adapters (public)   — Adapter interface + terraform plan adapter
evidra-infra (private)     — IaC, docker-compose, deploy scripts, CI
```

- Zero Go coupling between repos
- Communication: HTTP JSON (`POST /v1/validate`) only
- Adapters are stdin/stdout pipe binaries

### 3. Component Map (~40 lines)

Brief description of each component with its package path:

**API Server** (`cmd/evidra-api`)
- Phase 0: stateless, Ed25519 signing, static API key
- Phase 1: PostgreSQL, dynamic keys, usage tracking
- Phase 2: Skills API (feature-flagged)
- Key packages: `internal/api`, `internal/auth`, `internal/engine`, `internal/evidence`, `internal/storage`

**MCP Server** (`cmd/evidra-mcp`)
- Stdio transport for AI agents
- Online mode: delegates to API via `EVIDRA_URL`
- Offline mode: embedded OPA, local evidence
- Key packages: `pkg/mcpserver`

**CLI** (`cmd/evidra`)
- `evidra validate` — evaluates scenario files
- `evidra evidence` — inspects local evidence chain
- `evidra policy sim` — policy simulation
- Online mode: delegates to API. Offline: local OPA.
- Key packages: `pkg/validate`, `pkg/scenario`, `pkg/evidence`

**Shared Core** (`pkg/`)
- `pkg/validate` — scenario loader + OPA evaluation
- `pkg/invocation` — ToolInvocation, Actor types
- `pkg/evidence` — append-only store, hash linking
- `pkg/policy` + `pkg/runtime` — OPA engine wrapper
- `pkg/client` — HTTP client for API (online mode)
- `pkg/mode` — mode resolution (online/offline)
- `pkg/config` — environment/flag resolution, NormalizeEnvironment

**Input Adapters** (`evidra/adapters` repo)
- `evidra-adapter-terraform` — terraform show -json → ToolInvocation
- Interface: `Name()`, `Convert(ctx, raw, config) → Result`
- Separate Go module, goreleaser cross-compilation

### 4. Hybrid Mode (~30 lines)

Mode resolution diagram (the one from hybrid mode design §3):

```
EVIDRA_URL set?
├── NO → Offline
└── YES → Online (with fallbackPolicy)
```

Runtime behavior:
- Online: POST /v1/validate directly (no pre-ping)
- On 5xx/unreachable: fallbackPolicy decides (closed=error, offline=local eval)
- On 401/403/422/429: always error (no fallback)

Environment variables table: EVIDRA_URL, EVIDRA_API_KEY, EVIDRA_FALLBACK, EVIDRA_ENVIRONMENT

Exit codes: 0=allowed, 2=denied, 3=unreachable, 4=usage error

### 5. Policy Engine (~20 lines)

- OPA with `ops-v0.1` bundle (23 rules, embedded via go:embed)
- Evaluation: ToolInvocation → OPA input → allow/deny + risk_level + reasons
- Policy source: embedded bundle (default), custom bundle (--bundle), loose mode (--policy + --data)
- Environment-specific overrides via `by_env` in data.json

### 6. Evidence & Signing (~25 lines)

**Offline evidence:**
- Append-only JSONL in `~/.evidra/evidence/`
- Hash-linked chain (each record references previous hash)
- Segment files, manifest tracking

**Online evidence (API):**
- Ed25519 signed by server
- Signing payload: deterministic text format (length-prefixed encoding), NOT json.Marshal
- `signing_payload` field included in response for client-side verification
- `GET /v1/evidence/pubkey` returns public key (PEM)
- Evidence returned to client, never stored server-side

### 7. API Surface (quick reference, ~25 lines)

Table format:

| Phase | Method | Path | Auth | Description |
|---|---|---|---|---|
| P0 | POST | /v1/validate | Bearer | Policy evaluation → signed evidence |
| P0 | GET | /v1/evidence/pubkey | — | Ed25519 public key |
| P0 | GET | /healthz | — | Liveness |
| P1 | POST | /v1/keys | — (rate-limited) | Issue API key + create tenant |
| P1 | GET | /readyz | — | Readiness (DB connected) |
| P1 | POST | /v1/evidence/verify | Bearer (opt-in) | Server-side verification |
| P2 | POST | /v1/skills | Bearer | Register skill |
| P2 | POST | /v1/skills/{id}:execute | Bearer | Execute skill |
| P2 | POST | /v1/skills/{id}:simulate | Bearer | Dry-run skill |
| P2 | GET | /v1/executions/{id} | Bearer | Get execution record |

Key rule: **Deny = HTTP 200.** Check `decision.allow`, not HTTP status.

### 8. Deployment (~20 lines)

**Current (P0):**
- Hetzner CX22 (2 vCPU, 4 GB RAM, €4.67/mo)
- Traefik v3 reverse proxy, Let's Encrypt TLS
- Single `evidra-api` binary + embedded OPA bundle
- Domain: `evidra.rest`

**Future scaling path:**
- CX22 → CX32 → CX42 (vertical)
- Add PostgreSQL volume for Phase 1
- Kubernetes migration at scale (Phase 3)

### 9. Security Model (~15 lines)

- API keys: SHA-256 hashed, never stored plaintext
- Signing key: Ed25519, loaded from env var, never written to disk
- Evidence: client-side storage, server signs but doesn't store
- Input validation: reject \n \r in signing-payload fields
- Rate limiting: per key/IP, token bucket

---

## Writing Guidelines

- Use Mermaid diagrams where they add clarity (system overview, hybrid mode, dependency graph)
- Keep it factual — this is a reference doc, not marketing
- Mark future phases clearly: `[Phase 1]`, `[Phase 2]`
- Cross-reference: point to `docs/architecture_hosted-api-mvp.md` for full API details
- No emoji headers
- Target length: 200-300 lines
- This file should be what someone reads FIRST to understand Evidra's architecture
