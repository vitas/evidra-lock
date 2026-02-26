# Evidra — System Design for Claude Code Agent

**Date:** 2026-02-26
**Purpose:** Everything a Claude Code agent needs to create and develop the Evidra monorepo from scratch. Contains CLAUDE.md files, rules, project scaffold, and implementation guidance.

**How to use:** Open this document in Claude Code, then say: "Bootstrap the Evidra project using this system design."

---

## Part 1: Project Scaffold

Run once to create the directory structure:

```bash
mkdir -p evidra/{.claude/{rules,commands,context},pkg/{invocation,policy/bundles,evidence,version},api/{cmd/evidra-api,internal/{api,auth,engine,evidence,skills,storage,ratelimit,migrate/migrations}},mcp/{cmd/evidra-mcp,internal/{server,tools,transport}},cli/{cmd/evidra,internal/{commands,client,output}},adapters/{adapter,terraform,k8s,cmd/evidra-adapter-terraform},action,docs}
cd evidra
git init
```

---

## Part 2: Root CLAUDE.md

**File: `evidra/CLAUDE.md`**

```markdown
# Evidra

Policy evaluation + evidence signing for AI agent infrastructure operations.
Multi-module Go monorepo: shared core in `pkg/`, deployable modules in `api/`, `mcp/`, `cli/`, `adapters/`.

## What Evidra Does

An AI agent (Claude Code, Cursor, etc.) wants to run `kubectl apply` or `terraform apply`. Before executing, it calls Evidra. Evidra evaluates OPA policy against the operation, returns allow/deny + a cryptographically signed evidence record. The agent stores the evidence and proceeds (or aborts).

Key value: **non-repudiable proof** that a policy check happened, verifiable offline via Ed25519 public key.

## Module Map

| Module | Path | go.mod | Depends on | Builds |
|---|---|---|---|---|
| **Core** | `pkg/` | `github.com/evidra/evidra/pkg` | — | (library) |
| **API** | `api/` | `github.com/evidra/evidra/api` | `pkg/` | `evidra-api` binary |
| **MCP** | `mcp/` | `github.com/evidra/evidra/mcp` | `pkg/` | `evidra-mcp` binary |
| **CLI** | `cli/` | `github.com/evidra/evidra/cli` | `pkg/` | `evidra` binary |
| **Adapters** | `adapters/` | `github.com/evidra/evidra/adapters` | — (independent) | `evidra-adapter-terraform` |

**Dependency rule:** `api/`, `mcp/`, `cli/` all import `pkg/`. Nothing imports across `api/` ↔ `mcp/` ↔ `cli/`. Adapters have zero internal dependencies.

## Architecture Reference

Full architecture: `docs/architecture.md` (2200+ lines — read specific sections, not the whole file).

Key sections:
- §1: High-level architecture diagram and request flows
- §2: API surface — all endpoints with request/response shapes
- §3: Data model — PostgreSQL tables
- §4: Skill definition schema
- §4b: Input adapters — extraction layer design
- §5: Security — API keys, Ed25519 signing, signing payload format
- §6: Reuse of existing OPA engine
- §7: Code architecture — directory tree, dependency graph
- §12: Implementation plan — task sequence with DoD

## Launch Phases

| Phase | Gate | What works |
|---|---|---|
| **Phase 0** | No `DATABASE_URL` | Stateless: `/v1/validate`, `/v1/evidence/pubkey`, `/healthz`. Static API key. |
| **Phase 1** | `DATABASE_URL` set | + `/v1/keys`, `/readyz`. Dynamic key issuance, usage tracking. |
| **Phase 2** | `EVIDRA_SKILLS_ENABLED=true` | + `/v1/skills/*`, `/v1/executions/*`. |

**Start with Phase 0.** It has zero dependencies and proves the core value (policy check → signed evidence).

## Commands

```bash
# Build
make build-all              # all binaries → bin/
cd api && go build ./cmd/evidra-api
cd cli && go build ./cmd/evidra

# Test
make test-all               # all modules
cd pkg && go test ./...     # just core
cd api && go test ./...     # just API

# Lint
make lint                   # golangci-lint all modules

# Run (Phase 0 — no database needed)
EVIDRA_ENV=development EVIDRA_API_KEY="test-key-at-least-32-characters-long" bin/evidra-api

# Run (Phase 1 — with Postgres)
docker compose up -d postgres
DATABASE_URL="postgres://postgres:dev@localhost:5432/evidra?sslmode=disable" bin/evidra-api --migrate

# Test a request
curl -X POST http://localhost:8080/v1/validate \
  -H "Authorization: Bearer test-key-at-least-32-characters-long" \
  -H "Content-Type: application/json" \
  -d '{"actor":{"type":"agent","id":"claude","origin":"cli"},"tool":"kubectl","operation":"apply","params":{"target":{"namespace":"default"}}}'
```

## Conventions

- **Go 1.23+**, stdlib `net/http` (no frameworks), `encoding/json`, `crypto/ed25519`
- **No ORM** — raw `database/sql` + `pgx` driver
- **Errors** — `fmt.Errorf("module: %w", err)`, no custom error types unless needed for `errors.Is`
- **Logging** — `log/slog` (structured, JSON in prod, text in dev)
- **Tests** — table-driven, `t.Parallel()` where safe, `testcontainers-go` for integration tests with Postgres
- **Naming** — Go standard: `camelCase` locals, `PascalCase` exports, `snake_case` JSON/SQL
- **Context** — pass `context.Context` as first arg everywhere
- **No global state** — all dependencies injected via struct fields

## Cross-Module Change Rules

When modifying `pkg/`:
1. Check all downstream modules (api/, mcp/, cli/) for breakage
2. Run `make test-all` — NOT just `cd pkg && go test`
3. If adding a new export, consider whether downstream modules need it

**Never:**
- Import `api/internal/` from `mcp/` or `cli/`
- Put HTTP code in `pkg/`
- Put database code in `pkg/`
- Put business logic in `cmd/` (only wiring)
- Store evidence records server-side
- Write signing key material to disk

## Key Design Decisions

- **Deny = HTTP 200.** Policy deny is a successful evaluation, not an error. Check `decision.allow`, not HTTP status.
- **Evidence = client-side.** Server signs and returns evidence. Never stores it.
- **Signing payload = deterministic text, not JSON.** Length-prefixed encoding for lists.
- **input_hash = opaque.** Server-internal, not cross-version stable.
- **Adapters = external.** Evidra never parses terraform plans or k8s manifests in core.
```

---

## Part 3: Module-Specific CLAUDE.md Files

### File: `evidra/api/CLAUDE.md`

```markdown
# API Module — evidra-api

HTTP API server. Handles key issuance, policy evaluation, evidence signing.

## Build & Run
```bash
go build -o ../bin/evidra-api ./cmd/evidra-api
# Phase 0 (no DB):
EVIDRA_ENV=development EVIDRA_API_KEY="test-key-at-least-32-characters-long" ../bin/evidra-api
# Phase 1:
DATABASE_URL="postgres://..." ../bin/evidra-api --migrate
```

## Test
```bash
go test ./...                                    # all
go test -run TestValidateHandler ./internal/api/  # specific
go test -race ./...                              # race detector
```

## Handler Pattern

Every handler follows:
1. Parse request body (JSON unmarshal)
2. Validate input (structural validation, never trust client)
3. Call engine (via internal/engine adapter)
4. Build evidence record + sign with Ed25519
5. Increment usage counter
6. Return JSON response

```go
func (h *ValidateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Get tenant from context (set by auth middleware)
    tenantID := auth.TenantID(r.Context())

    // 2. Parse
    var inv invocation.ToolInvocation
    if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
        response.Error(w, http.StatusBadRequest, "invalid_json", err.Error())
        return
    }

    // 3. Evaluate
    result, err := h.engine.Evaluate(r.Context(), inv)

    // 4. Build + sign evidence
    rec := h.evidenceBuilder.Build(result, inv)
    h.signer.Sign(&rec)

    // 5. Usage
    h.usage.Increment(r.Context(), tenantID, "validate")

    // 6. Respond (always 200 — deny is not an error)
    response.JSON(w, http.StatusOK, ValidateResponse{...})
}
```

## Auth Modes

- **Phase 0:** `EVIDRA_API_KEY` env var → constant-time compare, synthetic tenant_id = "static"
- **Phase 1+:** SHA-256(bearer token) → DB lookup in api_keys → tenant_id from row

Both modes set tenant_id in context via `auth.WithTenantID(ctx, id)`.

## Response Format

All responses follow:
```json
{
  "ok": true,
  "event_id": "evt_01J...",
  "decision": {"allow": true, "risk_level": "low", ...},
  "evidence_record": {...}
}
```

Errors:
```json
{
  "ok": false,
  "error": {"code": "invalid_input", "message": "...", "details": {...}}
}
```

## Key Files
- `cmd/evidra-api/main.go` — wiring only: init config, DB, engine, signer, router, start server
- `internal/api/router.go` — stdlib http.ServeMux, mount all handlers
- `internal/api/validate_handler.go` — core policy evaluation endpoint
- `internal/auth/middleware.go` — dual-mode Bearer auth
- `internal/engine/adapter.go` — thin wrapper around pkg/policy
- `internal/storage/` — all PostgreSQL repository interfaces and implementations

## Config (env vars)
| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | Phase 1+ | — | Absent = Phase 0 stateless mode |
| `EVIDRA_API_KEY` | Phase 0 | — | Static key, ≥32 chars |
| `EVIDRA_SIGNING_KEY` | prod | — | Ed25519 base64 private key |
| `EVIDRA_ENV` | no | `production` | `development` = ephemeral signing key |
| `LISTEN_ADDR` | no | `:8080` | |
| `EVIDRA_VERIFY_ENABLED` | no | `false` | Enable POST /v1/evidence/verify |
| `EVIDRA_SKILLS_ENABLED` | no | `false` | Enable Phase 2 endpoints |
```

### File: `evidra/cli/CLAUDE.md`

```markdown
# CLI Module — evidra

Command-line client for Evidra API.

## Build
```bash
go build -o ../bin/evidra ./cmd/evidra
```

## Usage
```bash
evidra validate --file invocation.json
evidra validate --tool kubectl --operation apply --namespace prod --actor agent:claude
evidra keys create --label "my-pipeline"
evidra skills list
evidra skills execute scale-deployment --input '{"namespace":"prod","replicas":3}'
evidra evidence verify --file evidence.json --pubkey-url https://api.evidra.dev/v1/evidence/pubkey
```

## Architecture

Uses cobra for CLI framework. Each command in `internal/commands/`.

```
cmd/evidra/main.go          # root cobra command, add subcommands
internal/
  commands/
    validate.go             # evidra validate
    keys.go                 # evidra keys create|revoke
    skills.go               # evidra skills list|create|execute
    evidence.go             # evidra evidence verify
  client/
    client.go               # HTTP client for Evidra API (base URL, auth, retries)
    validate.go             # POST /v1/validate
    keys.go                 # POST /v1/keys
    skills.go               # skills CRUD + execute
  output/
    table.go                # Human-readable table output
    json.go                 # JSON output (--output json)
    yaml.go                 # YAML output (--output yaml)
```

## Conventions
- `--api-url` flag or `EVIDRA_API_URL` env var (default: `http://localhost:8080`)
- `--api-key` flag or `EVIDRA_API_KEY` env var
- `--output` flag: `table` (default), `json`, `yaml`
- Exit code 0 = success (even if policy denied), exit code 1 = error
- Evidence verify can work offline: download pubkey once, verify locally
```

### File: `evidra/mcp/CLAUDE.md`

```markdown
# MCP Module — evidra-mcp

MCP (Model Context Protocol) server that exposes Evidra policy evaluation as tools for AI agents.

## Build & Run
```bash
go build -o ../bin/evidra-mcp ./cmd/evidra-mcp
# Direct mode (calls pkg/ policy engine locally, no API server needed):
EVIDRA_ENV=development ../bin/evidra-mcp --transport stdio
# Proxy mode (calls Evidra API):
EVIDRA_API_URL=http://localhost:8080 EVIDRA_API_KEY=... ../bin/evidra-mcp --transport stdio
```

## Architecture

```
cmd/evidra-mcp/main.go          # init, transport selection
internal/
  server/
    server.go                    # MCP protocol handler (tools/list, tools/call)
    handlers.go                  # tool call routing
  tools/
    validate.go                  # evidra_validate tool
    registry.go                  # dynamic tool registration from skills (Phase 2)
  transport/
    stdio.go                     # stdin/stdout transport (for Claude Code, Cursor)
    sse.go                       # Server-Sent Events transport (for remote MCP)
```

## Modes

**Direct mode** (default when no `EVIDRA_API_URL`):
- Embeds `pkg/policy` engine directly
- No HTTP, no API server dependency
- Best for: Claude Code local use, MCP sidecar

**Proxy mode** (`EVIDRA_API_URL` set):
- Calls Evidra API over HTTP
- Skills-aware: reads skills from API, registers as MCP tools
- Best for: remote MCP, team shared server

## MCP Tools

Phase 0 — one generic tool:
```json
{
  "name": "evidra_validate",
  "description": "Evaluate infrastructure operation against policy. Returns allow/deny with signed evidence.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "tool": {"type": "string", "description": "Tool name: kubectl, terraform, aws, etc."},
      "operation": {"type": "string", "description": "Operation: apply, delete, plan, etc."},
      "target": {"type": "object", "description": "Target resource details"},
      "environment": {"type": "string"}
    },
    "required": ["tool", "operation"]
  }
}
```

Phase 2 — dynamic tools from skills (one MCP tool per registered skill):
```json
{
  "name": "evidra_k8s_deploy",
  "description": "Deploy to Kubernetes (policy-checked)",
  "inputSchema": { ... from skill.input_schema ... }
}
```

## Conventions
- MCP protocol: JSON-RPC 2.0 over transport
- Actor is auto-populated from MCP client metadata (if available) or defaults to `{"type":"agent","id":"mcp-client","origin":"mcp"}`
- Evidence records returned in tool call result for client-side storage
```

### File: `evidra/adapters/CLAUDE.md`

```markdown
# Adapters Module

Converts tool-specific output (terraform plan JSON, k8s manifests) into Evidra skill input parameters.

**This module has ZERO dependencies on pkg/, api/, mcp/, or cli/.** It only depends on external libraries (hashicorp/terraform-json, k8s.io/apimachinery).

## Build
```bash
go build -o ../bin/evidra-adapter-terraform ./cmd/evidra-adapter-terraform
```

## Usage
```bash
terraform show -json tfplan.bin | evidra-adapter-terraform
# outputs: {"create_count":2,"destroy_count":0,...}
```

## Interface
```go
type Adapter interface {
    Name() string
    Convert(ctx context.Context, raw []byte, config map[string]string) (*Result, error)
}

type Result struct {
    Input    map[string]any `json:"input"`
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

## Adding a New Adapter
1. Create `newtool/newtool.go` implementing `Adapter` interface
2. Add `cmd/evidra-adapter-newtool/main.go` (stdin → Convert → stdout JSON)
3. Write tests with sample tool output fixtures in `newtool/testdata/`
```

---

## Part 4: Rules Files

### File: `evidra/.claude/rules/go-style.md`

```markdown
# Go Style Rules

## Error Handling
- Always check errors. No `_` for error returns unless commented why.
- Wrap errors with context: `fmt.Errorf("storage.CreateKey: %w", err)`
- Use `errors.Is`/`errors.As` for error checking, not string comparison.
- Return early on error — no deep nesting.

## Naming
- Packages: short, lowercase, no underscores. `auth`, `storage`, `evidence`.
- Interfaces: verb-ish or noun. `Signer`, `Repository`, `Evaluator`. No `I` prefix.
- Exported functions: descriptive. `BuildSigningPayload`, not `Build` or `MakePayload`.
- Receivers: short (1-2 letters). `func (s *Signer) Sign(...)`, `func (h *Handler) ServeHTTP(...)`.

## Structure
- One type per file when the type is large (>50 lines). Multiple small types can share a file.
- `types.go` for pure data types with no methods.
- `cmd/` contains only wiring — no business logic.
- `internal/` for everything that shouldn't be importable.

## Dependencies
- Prefer stdlib. Only add dependencies when they provide significant value.
- Current approved deps: `github.com/open-policy-agent/opa`, `github.com/jackc/pgx/v5`, `github.com/oklog/ulid/v2`, `github.com/golang-migrate/migrate/v4`, `golang.org/x/crypto` (for constant-time compare).
- For CLI: `github.com/spf13/cobra`.
- For JSON Schema validation: `github.com/santhosh-tekuri/jsonschema/v5`.

## Formatting
- `gofmt` / `goimports` always.
- Line length: soft limit 100, hard limit 120.
- Comments: full sentences, starting with the name of the thing being described.
```

### File: `evidra/.claude/rules/testing.md`

```markdown
# Testing Rules

## Unit Tests
- File: `foo_test.go` next to `foo.go`.
- Table-driven tests for functions with multiple cases.
- Use `t.Parallel()` for independent tests.
- Test names: `TestFunctionName_Scenario` (`TestSign_ValidPayload`, `TestSign_EmptyField`).
- No test databases in unit tests — mock the interface.

## Integration Tests
- File: `foo_integration_test.go` with `//go:build integration` tag.
- Use `testcontainers-go` for Postgres.
- Test real SQL queries, not mocks.
- Run with: `go test -tags integration ./...`

## Test Helpers
- Common helpers in `testutil/` package within each module.
- `testutil.NewTestDB(t)` — starts Postgres container, runs migrations, returns *sql.DB.
- `testutil.MustSign(t, rec)` — sign evidence record with test key.

## What to Test
- Every exported function.
- Every HTTP handler (request → response, including error cases).
- Ed25519 sign/verify round-trip.
- Signing payload determinism (same input → same payload).
- Length-prefixed encoding with edge cases (commas, empty strings, unicode).
- Auth: both Phase 0 (static key) and Phase 1+ (DB lookup).
- Feature flags: disabled endpoints return 404.

## What NOT to Test
- Go stdlib functions.
- Simple getters/setters.
- main.go wiring (covered by integration/smoke tests).
```

### File: `evidra/.claude/rules/security.md`

```markdown
# Security Rules

## Signing Key
- Server NEVER writes key material to disk.
- Dev mode: ephemeral in-memory key + log warning.
- Prod: fail-fast (os.Exit) if no key provided.
- Use `crypto/ed25519` from stdlib. No external crypto libs.

## API Keys
- Generated with `crypto/rand` (256 bits entropy).
- Stored as SHA-256 hash — no salt needed (key has enough entropy).
- Plaintext returned ONCE in creation response, never stored.
- Constant-time compare for Phase 0 static key.
- On auth miss: constant-time sleep (50-100ms jitter) before returning 401.

## Evidence Records
- Never stored server-side.
- Signing payload is deterministic text, not JSON.
- Signature covers signing_payload field, not the whole record.
- Newlines and carriage returns rejected at input validation for fields that appear in signing payload.

## Logging
- NEVER log: API key plaintext, signing key material, full evidence records.
- OK to log: API key prefix (first 12 chars), tenant_id, event_id, decision.allow, tool, operation.
- Use `slog` with structured fields.

## Input Validation
- Reject `\n` and `\r` in actor.type, actor.id, actor.origin, tool, operation, environment.
- Max body size: 1MB (enforced by middleware).
- Max label length: 128 chars.
- Max input_schema size: 64KB.
```

### File: `evidra/.claude/rules/cross-module.md`

```markdown
# Cross-Module Rules

## pkg/ Changes
Any change to `pkg/` must be tested against ALL downstream modules:
```bash
make test-all  # NOT just: cd pkg && go test ./...
```

## Import Rules (STRICTLY ENFORCED)

✅ Allowed:
- `api/` → `pkg/`
- `mcp/` → `pkg/`
- `cli/` → `pkg/`

❌ Forbidden:
- `api/` → `mcp/internal/` or `cli/internal/`
- `mcp/` → `api/internal/` or `cli/internal/`
- `cli/` → `api/internal/` or `mcp/internal/`
- `pkg/` → `api/`, `mcp/`, `cli/`, `adapters/`
- `adapters/` → `pkg/`, `api/`, `mcp/`, `cli/`

## go.mod replace Directives
Each module that imports pkg/ uses:
```go
replace github.com/evidra/evidra/pkg => ../pkg
```
This is for local development ONLY. CI strips replace directives and uses tagged versions.

## Adding New Types to pkg/
1. Add type to `pkg/invocation/types.go` or `pkg/evidence/types.go`
2. If type appears in signing payload → update `evidence.BuildSigningPayload()`
3. If type is part of OPA input → update `policy/evaluate.go` input mapping
4. If type should be visible in CLI → update `cli/internal/output/`
5. Run `make test-all`
```

---

## Part 5: Go Module Files

### File: `evidra/pkg/go.mod`

```go
module github.com/evidra/evidra/pkg

go 1.23

require (
	github.com/oklog/ulid/v2 v2.1.0
	github.com/open-policy-agent/opa v1.4.2
)
```

### File: `evidra/api/go.mod`

```go
module github.com/evidra/evidra/api

go 1.23

require (
	github.com/evidra/evidra/pkg v0.0.0
	github.com/golang-migrate/migrate/v4 v4.18.1
	github.com/jackc/pgx/v5 v5.7.2
)

replace github.com/evidra/evidra/pkg => ../pkg
```

### File: `evidra/cli/go.mod`

```go
module github.com/evidra/evidra/cli

go 1.23

require (
	github.com/evidra/evidra/pkg v0.0.0
	github.com/spf13/cobra v1.8.1
)

replace github.com/evidra/evidra/pkg => ../pkg
```

### File: `evidra/mcp/go.mod`

```go
module github.com/evidra/evidra/mcp

go 1.23

require (
	github.com/evidra/evidra/pkg v0.0.0
)

replace github.com/evidra/evidra/pkg => ../pkg
```

### File: `evidra/adapters/go.mod`

```go
module github.com/evidra/evidra/adapters

go 1.23

require (
	github.com/hashicorp/terraform-json v0.24.0
)
```

---

## Part 6: Makefile

### File: `evidra/Makefile`

```makefile
MODULES := pkg api mcp cli adapters
BINDIR  := bin

.PHONY: build-all test-all lint clean scaffold

build-all: $(BINDIR)
	cd api && go build -o ../$(BINDIR)/evidra-api ./cmd/evidra-api
	cd mcp && go build -o ../$(BINDIR)/evidra-mcp ./cmd/evidra-mcp
	cd cli && go build -o ../$(BINDIR)/evidra ./cmd/evidra
	cd adapters && go build -o ../$(BINDIR)/evidra-adapter-terraform ./cmd/evidra-adapter-terraform

$(BINDIR):
	mkdir -p $(BINDIR)

test-all:
	@for mod in $(MODULES); do \
		echo "=== Testing $$mod ===" && \
		(cd $$mod && go test ./...) || exit 1; \
	done

test-race:
	@for mod in $(MODULES); do \
		echo "=== Race test $$mod ===" && \
		(cd $$mod && go test -race ./...) || exit 1; \
	done

lint:
	@for mod in $(MODULES); do \
		echo "=== Linting $$mod ===" && \
		(cd $$mod && golangci-lint run ./...) || exit 1; \
	done

clean:
	rm -rf $(BINDIR)

# Quick Phase 0 start (no database)
run-phase0:
	EVIDRA_ENV=development \
	EVIDRA_API_KEY="evidra-dev-key-minimum-32-characters" \
	go run ./api/cmd/evidra-api

# Phase 1 start (requires running Postgres)
run-phase1:
	DATABASE_URL="postgres://postgres:dev@localhost:5432/evidra?sslmode=disable" \
	EVIDRA_ENV=development \
	go run ./api/cmd/evidra-api --migrate
```

---

## Part 7: Docker Compose

### File: `evidra/docker-compose.yml`

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: evidra
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: dev
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 3s
      retries: 5

  api:
    build:
      context: .
      dockerfile: api/Dockerfile
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://postgres:dev@postgres:5432/evidra?sslmode=disable
      EVIDRA_ENV: development
      LISTEN_ADDR: ":8080"
    ports:
      - "8080:8080"
    command: ["/usr/local/bin/evidra-api", "--migrate"]

volumes:
  pgdata:
```

---

## Part 8: Implementation Order

**Start here. Phase 0 first — zero dependencies, proves core value.**

### Step 1: pkg/ Core Types (1-2 hours)

```
pkg/invocation/types.go     — ToolInvocation, Actor structs + JSON tags
pkg/invocation/validate.go  — ValidateStructure() — reject \n\r, check required fields
pkg/evidence/types.go       — EvidenceRecord, DecisionRecord structs
pkg/version/version.go      — build info vars
```

### Step 2: pkg/ Evidence Signing (2-3 hours)

```
pkg/evidence/signer.go      — Signer struct, key loading, Sign(), Verify()
pkg/evidence/payload.go     — BuildSigningPayload(), lengthPrefixedJoin()
pkg/evidence/builder.go     — BuildRecord(result, invocation) → EvidenceRecord
```

Test: sign → verify round-trip, length-prefixed encoding edge cases.

### Step 3: pkg/ Policy Engine (1-2 hours)

```
pkg/policy/engine.go        — OPA engine init, lifecycle
pkg/policy/evaluate.go      — EvaluateInvocation() → Decision
pkg/policy/bundles/         — copy ops-v0.1 bundle here (embed)
```

Test: evaluate known allow + deny scenarios against ops-v0.1 policy.

### Step 4: api/ Phase 0 Server (3-4 hours)

```
api/cmd/evidra-api/main.go          — detect mode, init, start
api/internal/auth/middleware.go      — Phase 0: constant-time compare
api/internal/auth/context.go         — tenant context get/set
api/internal/engine/adapter.go       — wrap pkg/policy
api/internal/api/validate_handler.go — POST /v1/validate
api/internal/api/verify_handler.go   — GET /v1/evidence/pubkey
api/internal/api/health_handler.go   — GET /healthz
api/internal/api/router.go           — mount handlers
api/internal/api/response.go         — JSON helpers
```

Test: start server → `POST /v1/validate` → signed evidence → verify with pubkey.

**At this point you have a working Phase 0.** A Claude Code agent can call it via MCP.

### Step 5: api/ Phase 1 Storage (2-3 hours)

```
api/internal/storage/postgres.go    — pool init
api/internal/migrate/migrations/    — 001_initial.up.sql
api/internal/storage/tenants.go     — TenantRepo
api/internal/storage/apikeys.go     — APIKeyRepo
api/internal/storage/usage.go       — UsageRepo
api/internal/auth/middleware.go      — add Phase 1 DB lookup mode
api/internal/api/keys_handler.go     — POST /v1/keys
```

### Step 6: cli/ Basic Commands (2-3 hours)

```
cli/cmd/evidra/main.go
cli/internal/client/client.go       — HTTP client
cli/internal/commands/validate.go    — evidra validate
cli/internal/commands/evidence.go    — evidra evidence verify
cli/internal/output/table.go
```

### Step 7: mcp/ Server (3-4 hours)

```
mcp/cmd/evidra-mcp/main.go
mcp/internal/server/server.go        — MCP protocol
mcp/internal/tools/validate.go       — evidra_validate tool
mcp/internal/transport/stdio.go      — stdin/stdout
```

### Steps 8+: Phase 2 (Skills), Adapters, GitHub Action — see architecture doc §12.

---

## Part 9: Context Files (For Reading on Demand)

### File: `evidra/.claude/context/api-surface.md`

This is a condensed reference of all endpoints. Claude Code should read this (not the full architecture doc) when working on handlers or client code.

```markdown
# API Surface — Quick Reference

## Phase 0
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | /v1/validate | Bearer | Evaluate policy → signed evidence |
| GET | /v1/evidence/pubkey | — | Ed25519 public key (PEM) |
| GET | /healthz | — | Always 200 |

## Phase 1
| POST | /v1/keys | — (rate-limited) | Issue API key + tenant |
| GET | /readyz | — | 200 if DB connected |
| POST | /v1/evidence/verify | Bearer (opt-in) | Server-side signature verification |

## Phase 2
| POST | /v1/skills | Bearer | Register skill |
| GET | /v1/skills | Bearer | List tenant's skills |
| GET | /v1/skills/{id} | Bearer | Get skill |
| PUT | /v1/skills/{id} | Bearer | Update skill |
| DELETE | /v1/skills/{id} | Bearer | Soft-delete skill |
| POST | /v1/skills/{id}:simulate | Bearer | Dry-run (no evidence, no execution record) |
| POST | /v1/skills/{id}:execute | Bearer | Execute (signed evidence + execution record) |
| GET | /v1/executions/{id} | Bearer | Get execution metadata |

## Key Rules
- Deny = HTTP 200 (not 403). Check `response.decision.allow`.
- All authed endpoints require `Authorization: Bearer ev1_...`
- Evidence records are in response body, never stored server-side.
- `/v1/evidence/verify` returns 404 unless `EVIDRA_VERIFY_ENABLED=true`.
- `/v1/skills/*` returns 404 unless `EVIDRA_SKILLS_ENABLED=true`.
```

### File: `evidra/.claude/context/data-model.md`

```markdown
# Data Model — Quick Reference

## tenants
| Column | Type | Notes |
|---|---|---|
| id | TEXT PK | ULID |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() |

## api_keys
| Column | Type | Notes |
|---|---|---|
| id | TEXT PK | ULID |
| tenant_id | TEXT FK→tenants | NOT NULL |
| key_hash | BYTEA UNIQUE | SHA-256 of plaintext key |
| prefix | TEXT | First 12 chars for log correlation |
| label | TEXT | Optional, max 128 |
| created_at | TIMESTAMPTZ | |
| last_used_at | TIMESTAMPTZ | Updated on each auth |
| revoked_at | TIMESTAMPTZ | NULL = active |

## usage_counters
| Column | Type | Notes |
|---|---|---|
| tenant_id | TEXT | Part of composite PK |
| endpoint | TEXT | "validate", "execute", etc. |
| bucket | TEXT | "2026-02-26" (daily) |
| count | BIGINT | Upsert increment |

## skills (Phase 2)
| Column | Type | Notes |
|---|---|---|
| id | TEXT PK | "sk_" + ULID |
| tenant_id | TEXT FK | |
| name | TEXT | UNIQUE per tenant |
| tool | TEXT | |
| operation | TEXT | |
| input_schema | JSONB | JSON Schema draft-07 |
| risk_tags | TEXT[] | |
| default_environment | TEXT | |
| default_target | JSONB | |
| deleted_at | TIMESTAMPTZ | Soft delete |

## executions (Phase 2)
| Column | Type | Notes |
|---|---|---|
| id | TEXT PK | "exec_" + ULID |
| tenant_id | TEXT FK | |
| skill_id | TEXT FK | |
| event_id | TEXT | From evidence record |
| idempotency_key | TEXT UNIQUE | Client-provided |
| allow | BOOLEAN | |
| created_at | TIMESTAMPTZ | |
```

---

## Part 10: .gitignore

### File: `evidra/.gitignore`

```
bin/
*.exe
.env
.env.*
*.pem
*.key
vendor/
.idea/
.vscode/
*.DS_Store
```

---

## Summary

This document gives Claude Code everything it needs:
1. **CLAUDE.md hierarchy** — root + module-specific = always the right context
2. **Rules** — go style, testing, security, cross-module boundaries
3. **Context files** — API surface, data model — read on demand, not always loaded
4. **Go module setup** — multi-module monorepo with replace directives
5. **Makefile + Docker Compose** — build, test, run commands
6. **Implementation order** — Phase 0 first, ~15 hours to working API + CLI + MCP

Start Claude Code at the repo root. Say "Let's implement Step 1: pkg/ core types." Follow the order. Each step is self-contained and testable.
