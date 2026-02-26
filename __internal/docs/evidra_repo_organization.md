# Evidra — Repository Organization & AI Development Workflow

**Date:** 2026-02-26
**Status:** Decision record
**Scope:** How to split code across repos, share a core, and work effectively with Claude Code

---

## Decision: Go Multi-Module Monorepo

**Not** multiple repositories. **Not** a classic monorepo with a single `go.mod`. A **multi-module monorepo** — one git repo, multiple Go modules, each independently versionable and buildable.

### Why Not Multi-Repo

Multi-repo (separate git repos for backend, mcp, cli, adapters) has one fatal problem for a solo/small-team AI-assisted workflow: **context fragmentation**. When Claude Code works on the MCP server and needs to understand the core types, it has to `--add-dir` another repo or clone it — extra setup, token waste, context window pollution. Every cross-repo change becomes two PRs, two CI runs, and manual version bumping.

With a monorepo, Claude Code sees everything. One `CLAUDE.md`, one context, atomic cross-module changes in a single commit.

### Why Not Single-Module Monorepo

A single `go.mod` for everything means the CLI depends on the PostgreSQL driver, the MCP server imports the HTTP router, and anyone who `go install`s the CLI pulls 50 transitive dependencies they don't need. Go's design strongly favors one module = one deployable unit.

### Multi-Module Monorepo: Best of Both

One git repo. Each deployable component is a separate Go module with its own `go.mod`. Shared code lives in a `pkg/` module that all others import via `replace` directives (local development) and tagged versions (releases).

```
evidra/
├── CLAUDE.md                    # Root instructions for Claude Code
├── .claude/
│   ├── rules/
│   │   ├── go-style.md          # Go coding standards
│   │   ├── testing.md           # Test conventions
│   │   └── cross-module.md      # Rules for cross-module changes
│   └── context/
│       ├── architecture.md      # → symlink or copy of architecture doc
│       └── decisions.md         # ADR log
├── Makefile                     # Top-level: build all, test all, lint all
├── docker-compose.yml           # Local dev: postgres, api, mcp
│
├── pkg/                         # ── Shared core module ──
│   ├── go.mod                   # module github.com/evidra/evidra/pkg
│   ├── go.sum
│   ├── invocation/
│   │   ├── types.go             # ToolInvocation, Actor, Decision
│   │   └── validate.go          # Structural validation
│   ├── policy/
│   │   ├── engine.go            # OPA engine wrapper
│   │   ├── bundles/             # Embedded policy bundles
│   │   └── evaluate.go          # EvaluateInvocation()
│   ├── evidence/
│   │   ├── signer.go            # Ed25519 signing
│   │   ├── builder.go           # Build EvidenceRecord
│   │   └── types.go             # EvidenceRecord, signing payload
│   └── version/
│       └── version.go           # Build info, injected at compile time
│
├── api/                         # ── Backend API module ──
│   ├── go.mod                   # module github.com/evidra/evidra/api
│   ├── go.sum                   # require ../pkg (replace directive)
│   ├── cmd/
│   │   └── evidra-api/
│   │       └── main.go
│   ├── internal/
│   │   ├── api/                 # HTTP handlers
│   │   ├── auth/                # API key middleware
│   │   ├── storage/             # PostgreSQL repos
│   │   ├── skills/              # Skill builder, validator
│   │   ├── ratelimit/
│   │   └── migrate/
│   ├── Dockerfile
│   └── CLAUDE.md                # API-specific instructions
│
├── mcp/                         # ── MCP Server module ──
│   ├── go.mod                   # module github.com/evidra/evidra/mcp
│   ├── go.sum                   # require ../pkg (replace directive)
│   ├── cmd/
│   │   └── evidra-mcp/
│   │       └── main.go
│   ├── internal/
│   │   ├── server/              # MCP protocol handler
│   │   ├── tools/               # Tool registration from skills
│   │   └── transport/           # stdio, SSE transports
│   ├── Dockerfile
│   └── CLAUDE.md                # MCP-specific instructions
│
├── cli/                         # ── CLI module ──
│   ├── go.mod                   # module github.com/evidra/evidra/cli
│   ├── go.sum                   # require ../pkg (replace directive)
│   ├── cmd/
│   │   └── evidra/
│   │       └── main.go
│   ├── internal/
│   │   ├── commands/            # validate, execute, keys, skills
│   │   ├── client/              # HTTP client for Evidra API
│   │   └── output/              # Table, JSON, YAML formatters
│   └── CLAUDE.md                # CLI-specific instructions
│
├── adapters/                    # ── Input Adapters module ──
│   ├── go.mod                   # module github.com/evidra/evidra/adapters
│   ├── go.sum                   # NO dependency on ../pkg — fully independent
│   ├── adapter/
│   │   └── adapter.go           # Interface + Result type
│   ├── terraform/
│   │   └── plan.go              # terraform show -json → skill input
│   ├── k8s/
│   │   └── manifest.go
│   ├── cmd/
│   │   └── evidra-adapter-terraform/
│   │       └── main.go
│   └── CLAUDE.md
│
├── action/                      # ── GitHub Action ──
│   ├── action.yml
│   ├── entrypoint.sh
│   └── CLAUDE.md
│
└── docs/
    ├── architecture.md          # Full architecture doc
    ├── client-evidence-guide.md
    └── skills-integration.md
```

### Module Dependency Graph

```
adapters  ──(independent)──  no internal deps, only external libs

cli ───────→ pkg
mcp ───────→ pkg
api ───────→ pkg

action ────→ cli (uses evidra CLI binary) + adapters (optional)
```

Key: `pkg` is the shared core. Everything imports `pkg`, nothing imports `api`, `mcp`, `cli`, or `adapters` across module boundaries.

### Local Development with `replace`

Each module's `go.mod` uses a `replace` directive for local development:

```go
// api/go.mod
module github.com/evidra/evidra/api

go 1.23

require (
    github.com/evidra/evidra/pkg v0.0.0
)

replace github.com/evidra/evidra/pkg => ../pkg
```

This means: `go build`, `go test`, `go run` all work instantly with local changes to `pkg/`. No version tagging needed during development. When releasing, the `replace` is stripped and a tagged version of `pkg` is used.

### Releasing

```bash
# Tag shared core first
git tag pkg/v0.1.0
git push origin pkg/v0.1.0

# Then tag each module that depends on it
git tag api/v0.1.0
git tag mcp/v0.1.0
git tag cli/v0.1.0
git push origin api/v0.1.0 mcp/v0.1.0 cli/v0.1.0
```

CI automation: a `release.sh` script that tags all modules in dependency order.

---

## Claude Code Workflow: Multi-Module Monorepo

### CLAUDE.md Hierarchy

Claude Code reads `CLAUDE.md` from the working directory and parent directories. This gives us a natural layering:

```
evidra/CLAUDE.md                 # Global: project overview, module map, shared conventions
evidra/api/CLAUDE.md             # API-specific: handler patterns, DB conventions, test commands
evidra/mcp/CLAUDE.md             # MCP-specific: protocol details, transport patterns
evidra/cli/CLAUDE.md             # CLI-specific: cobra patterns, output formatting
evidra/adapters/CLAUDE.md        # Adapters: interface contract, test patterns
```

**Root `CLAUDE.md`** (always loaded) contains:

```markdown
# Evidra

Multi-module Go monorepo. Policy evaluation + evidence signing for AI agent infrastructure operations.

## Module Map
- `pkg/` — Shared core: types, policy engine, evidence signer. NO HTTP, NO database.
- `api/` — HTTP API server. Depends on pkg/. Has PostgreSQL storage.
- `mcp/` — MCP server for AI agents. Depends on pkg/. Stateless.
- `cli/` — CLI client. Depends on pkg/. Calls api/ over HTTP.
- `adapters/` — Input adapters (terraform, k8s). NO dependency on pkg/.
- `action/` — GitHub Action. Shell wrapper around cli + adapters.

## Rules
- Changes to `pkg/` MUST NOT break api/, mcp/, or cli/. Run `make test-all` after pkg/ changes.
- Each module has its own `go.mod`. Use `replace` directives for local dev.
- Never import `api/internal/`, `mcp/internal/`, `cli/internal/` from another module.
- Tests: `cd <module> && go test ./...` or `make test-all` from root.
- Lint: `make lint` from root runs golangci-lint on all modules.

## Architecture
See docs/architecture.md for full design. Key sections:
- §2: API Surface (all endpoints)
- §4: Skill Definition Schema
- §4b: Input Adapters
- §5: Security Design (signing, keys)
- §7: Code Architecture (dependency graph)

## Common Commands
- `make build-all` — build all binaries
- `make test-all` — test all modules
- `make lint` — lint all modules
- `make docker-up` — start local dev environment (postgres + api)
- `cd api && go run ./cmd/evidra-api` — run API server locally
- `cd mcp && go run ./cmd/evidra-mcp` — run MCP server locally
```

**Module-specific `CLAUDE.md`** (loaded when cd'd into module):

```markdown
# API Module

## Build & Test
- `go run ./cmd/evidra-api` — start server (needs DATABASE_URL or runs Phase 0)
- `go test ./...` — run all tests
- `go test -run TestValidate ./internal/api/` — run specific handler tests

## Conventions
- Handlers in internal/api/ follow: parse → validate → call engine → sign → respond
- All DB access through repository interfaces in internal/storage/
- Never import from mcp/, cli/, or adapters/
- Error responses use response.Error(w, code, msg, details)

## Key Files
- internal/api/validate_handler.go — main policy evaluation endpoint
- internal/engine/adapter.go — thin wrapper around pkg/policy
- internal/evidence/signer.go — Ed25519 signing
```

### Working With Multiple Modules Simultaneously

**Scenario 1: Change to `pkg/` that affects `api/` and `mcp/`**

```bash
# Work from root — Claude sees everything
cd ~/code/evidra
claude

# Claude can: edit pkg/invocation/types.go,
# update api/internal/api/validate_handler.go,
# update mcp/internal/tools/registry.go,
# run tests across all modules
```

Claude Code from the monorepo root has visibility into all modules. The root `CLAUDE.md` tells it the dependency graph, so it knows that pkg/ changes require testing downstream modules.

**Scenario 2: Focused work on one module**

```bash
# Work from module dir — Claude sees module + root CLAUDE.md
cd ~/code/evidra/mcp
claude

# Claude focuses on MCP, but can still read ../pkg/ if needed
```

**Scenario 3: Parallel work with worktrees**

```bash
# Main session: working on API features
cd ~/code/evidra
claude --worktree api-skills-phase2

# Second session: MCP tool registration (separate branch)
claude --worktree mcp-dynamic-tools

# Third session: CLI output formatting (separate branch)
claude --worktree cli-json-output
```

Each worktree has a full copy of the monorepo, so each Claude session can see all modules. Branches are independent — no conflicts between sessions.

### .claude/rules/ — Cross-Module Rules

```
.claude/rules/
├── go-style.md              # Coding standards (error handling, naming, etc.)
├── testing.md               # Test file naming, table-driven tests, mocks
├── cross-module.md          # Rules for changes that span modules
├── security.md              # Key handling, signing, no-disk-write rules
└── api-design.md            # HTTP conventions, response format, status codes
```

**`cross-module.md`** — the critical one:

```markdown
# Cross-Module Change Rules

When modifying `pkg/`:
1. Check all downstream modules: api/, mcp/, cli/
2. Update any code that uses changed types or functions
3. Run `make test-all` from root to verify nothing breaks
4. If adding a new export to pkg/, consider whether api/, mcp/, cli/ need it

When adding a new type to `pkg/invocation/`:
1. Update evidence builder if the type affects signing payload
2. Update OPA input mapping in pkg/policy/evaluate.go
3. Check if CLI output formatters need updating

Never:
- Import api/internal/ from mcp/ or cli/
- Import mcp/internal/ from api/ or cli/
- Put HTTP or database code in pkg/
- Put business logic in cmd/ (only wiring)
```

### Context Management for Long Sessions

For a project with 2000+ lines of architecture docs, context window management matters.

**Strategy: Layered context, not monolithic.**

1. **CLAUDE.md** contains the module map and key commands — always in context (small, ~50 lines)
2. **docs/architecture.md** is NOT in CLAUDE.md — Claude reads it only when needed (`read docs/architecture.md`)
3. **.claude/context/** contains focused summaries for specific topics:
   - `context/api-surface.md` — endpoint list with request/response shapes
   - `context/data-model.md` — table schemas
   - `context/security.md` — signing and auth summary
4. Module-specific CLAUDE.md files contain only what's needed for that module

**When Claude needs architecture context**, it reads the relevant context file, not the entire 2000-line doc. This keeps context lean.

**Use `/compact` aggressively.** After completing a task, compact. Before starting a new task in a different module, compact. The monorepo root CLAUDE.md is always reloaded after compact, so Claude never loses the project map.

### Subagents for Multi-Module Tasks

Claude Code subagents are ideal for multi-module work:

```
Main agent (root):
  "Add risk_level field to EvidenceRecord"

  → Subagent 1: "Update pkg/evidence/types.go — add RiskLevel string field"
  → Subagent 2: "Update api/internal/api/validate_handler.go — populate RiskLevel from decision"
  → Subagent 3: "Update cli/internal/output/evidence.go — display RiskLevel in table output"
  → Main: run make test-all, verify
```

The main agent orchestrates, subagents handle focused module changes. Each subagent inherits the root CLAUDE.md context.

---

## CI/CD for Multi-Module Monorepo

### Makefile (root)

```makefile
MODULES := pkg api mcp cli adapters

.PHONY: test-all build-all lint

test-all:
	@for mod in $(MODULES); do \
		echo "=== Testing $$mod ===" && \
		cd $$mod && go test ./... && cd .. || exit 1; \
	done

build-all:
	cd api && go build -o ../bin/evidra-api ./cmd/evidra-api
	cd mcp && go build -o ../bin/evidra-mcp ./cmd/evidra-mcp
	cd cli && go build -o ../bin/evidra ./cmd/evidra
	cd adapters && go build -o ../bin/evidra-adapter-terraform ./cmd/evidra-adapter-terraform

lint:
	@for mod in $(MODULES); do \
		echo "=== Linting $$mod ===" && \
		cd $$mod && golangci-lint run ./... && cd .. || exit 1; \
	done
```

### GitHub Actions — Path-Filtered CI

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  detect-changes:
    runs-on: ubuntu-latest
    outputs:
      pkg: ${{ steps.filter.outputs.pkg }}
      api: ${{ steps.filter.outputs.api }}
      mcp: ${{ steps.filter.outputs.mcp }}
      cli: ${{ steps.filter.outputs.cli }}
      adapters: ${{ steps.filter.outputs.adapters }}
    steps:
      - uses: dorny/paths-filter@v3
        id: filter
        with:
          filters: |
            pkg: 'pkg/**'
            api: ['api/**', 'pkg/**']      # api also runs on pkg changes
            mcp: ['mcp/**', 'pkg/**']      # mcp also runs on pkg changes
            cli: ['cli/**', 'pkg/**']      # cli also runs on pkg changes
            adapters: 'adapters/**'         # adapters is independent

  test-pkg:
    needs: detect-changes
    if: needs.detect-changes.outputs.pkg == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: cd pkg && go test ./...

  test-api:
    needs: detect-changes
    if: needs.detect-changes.outputs.api == 'true'
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env: { POSTGRES_PASSWORD: test }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: cd api && go test ./...

  # ... similar for mcp, cli, adapters
```

Key: **pkg changes trigger tests in all downstream modules.** Adapter changes only trigger adapter tests.

---

## Summary: Why This Works for AI-Driven Development

| Concern | Multi-repo pain | Monorepo solution |
|---|---|---|
| **Context** | Claude Code needs `--add-dir` for each repo, wastes tokens | One working directory, full visibility |
| **Atomic changes** | Two PRs for one feature, manual coordination | One commit, one PR |
| **Type sharing** | Published Go module, version lag, import cycle risk | `replace` directive, instant iteration |
| **CLAUDE.md** | Separate per repo, no global view | Hierarchical: root + module-specific |
| **Testing** | Run tests manually across repos | `make test-all` from root |
| **Worktrees** | One worktree per repo = 4 worktrees for one feature | One worktree = full project |
| **Subagents** | Cannot edit across repo boundaries | Full access to all modules |
| **CI** | Separate pipelines, no cross-repo triggers | Path-filtered, pkg triggers downstream |
| **Release** | Manual version bumping across repos | Tag in order: pkg → api/mcp/cli |

The multi-module monorepo gives you: independent builds (each module compiles alone), shared types (import `pkg/` with `replace`), one context window for Claude Code, and atomic cross-module changes. It's the ideal structure for a solo developer working with AI agents.
