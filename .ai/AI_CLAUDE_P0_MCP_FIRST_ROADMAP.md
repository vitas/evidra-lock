# P0 — MCP-First Plan
Generated: 2026-02-25
Source prompt: product_owner_feature_proposal_road_map.md (strategic shift: MCP-first AI safety backend)

---

## Strategic Context

Evidra is repositioned as: **MCP-first AI safety backend for infrastructure agents.**

Gating question for P0:
> Can a developer connect Evidra to an AI agent in under 5 minutes and see it block a dangerous action?

If not — it's P0. Everything else is P0.1+.

**P0 scope:** zero-config `evidra-mcp` startup, install path for the MCP binary, MCP-first README + demo, 3-minute quickstart.

**Explicitly excluded from P0:** policy library expansion, GitHub Action, coverage tooling, enterprise signals, HTTP transport (→ P1), evidence export polish, `list_rules`/`simulate` tools, compliance or SaaS features, CI hardening, binary size verification, release pipeline hardening (→ P0.1).

---

## P0 — Critical (Adoption blockers)

> Gating question: Can a developer connect Evidra to an AI agent in under 5 minutes and see it block a dangerous action? If not — it belongs here.

---

### 1. Embed the bundle — zero-config `evidra-mcp` startup

**Why it matters:** `evidra-mcp` fails on any machine that didn't clone the repo. Every install path is broken until the bundle ships inside the binary.

**Complexity:** Medium
**Adoption impact:** High
**Owner:** MCP

**Definition of Done:**
- `evidra-mcp` starts with zero flags on a clean machine and accepts MCP tool calls.
- stderr emits `using built-in ops-v0.1 bundle (override with --bundle or EVIDRA_BUNDLE_PATH)`.
- `docker run ghcr.io/<org>/evidra-mcp` starts and serves `validate` with no mounts or env vars.

**Execution Plan:**
1. Add `//go:embed policy/bundles/ops-v0.1` in `cmd/evidra-mcp/main.go`; use `embed.FS`.
2. Write `extractEmbeddedBundle(fs embed.FS, destDir string) error` in `cmd/evidra-mcp/` — binary concern only.
3. Extend bundle path resolution in `pkg/config`: final fallback calls extract into `os.MkdirTemp`, caches path for process lifetime.
4. Print the single-line stderr notice on embedded fallback.
5. Update `Dockerfile.mcp`: `FROM gcr.io/distroless/static`, copy binary only — no bundle volume.

---

### 2. Install path: `evidra-mcp` first (Homebrew + Docker)

**Why it matters:** No Homebrew formula and no Docker image means every install requires cloning and building. Developers need one command to get a running MCP server.

**Complexity:** Medium
**Adoption impact:** High
**Owner:** Release

**Definition of Done:**
- `brew install <org>/evidra/evidra-mcp` installs a working binary; starts with zero flags.
- `docker run ghcr.io/<org>/evidra-mcp:latest` starts the MCP server — no mounts, no env vars.
- README install block links to working Homebrew tap and GHCR image; no placeholder text.

**Execution Plan:**
1. Create `homebrew-evidra` tap repo; add stub formula with placeholder archive URL.
2. Add `brews` block to `.goreleaser.yaml`; `install: bin.install "evidra-mcp"`. CLI is secondary.
3. Write `Dockerfile.mcp`: `FROM gcr.io/distroless/static`, copy binary, `ENTRYPOINT ["/evidra-mcp"]`.
4. Add `dockers` block to `.goreleaser.yaml` for `ghcr.io/<org>/evidra-mcp`; tag `{{ .Tag }}` and `latest`.
5. Add GHCR login step to `.github/workflows/release.yml` using `GITHUB_TOKEN`.
6. Cut `v0.1.0` tag; confirm Homebrew formula updates and image appears on GHCR.

---

### 3. MCP-first README and demo

**Why it matters:** The current README leads with architecture. A developer needs to see install → config block → AI agent blocked, in under 90 seconds of reading.

**Complexity:** Low
**Adoption impact:** High
**Owner:** Docs

**Definition of Done:**
- README opens with positioning line and terminal GIF (≤ 3 MB) before any prose.
- GIF shows: `validate` → PASS, then `validate` → FAIL with hint returned.
- Install block within first 20 lines; Homebrew and Docker paths only — no `git clone`.
- Copy-pasteable `claude_desktop_config.json` block is present and syntactically valid.
- No internal package paths (`pkg/`, `cmd/`) appear in the README.

**Execution Plan:**
1. Confirm `examples/terraform_plan_pass.json` and `examples/terraform_deny_mass_delete.json` exist and produce expected PASS/FAIL.
2. Write `demo.tape` for `vhs`: start `evidra-mcp`, PASS call, FAIL call, show hints.
3. Record GIF; keep under 3 MB.
4. Rewrite README: positioning → GIF → install → Claude Desktop config block → quickstart link → docs link.
5. Write and test `claude_desktop_config.json` against an installed binary; include exact `command` and `args`.
6. Move architecture, package table, and internals to `docs/architecture.md`; link from README footer only.

---

### 4. 3-minute MCP quickstart: connect, call, see a block

**Why it matters:** There is no path from zero to "AI agent blocked by Evidra" in under 5 minutes. Without it the product cannot be evaluated. Developers close the tab.

**Complexity:** Low
**Adoption impact:** High
**Owner:** Docs

**Definition of Done:**
- `docs/quickstart-mcp.md` covers: install → configure Claude Desktop → PASS call → FAIL call with hints — completable in under 3 minutes.
- Every command is copy-pasteable and produces the documented output on a clean macOS machine.
- FAIL scenario uses `examples/terraform_deny_mass_delete.json` and shows exact hint text from the bundle.
- "What just happened?" section explains the evidence record and how to retrieve it with `get_event`.
- README links to `docs/quickstart-mcp.md` from the install block.

**Execution Plan:**
1. Write `docs/quickstart-mcp.md`: Prerequisites → Install → Configure Claude Desktop → PASS scenario → FAIL scenario → Inspect evidence.
2. Confirm `examples/terraform_deny_mass_delete.json` triggers `ops.mass_delete` with the embedded bundle.
3. Include the exact `validate` input JSON so the quickstart works without a real Terraform plan.
4. Write "What just happened?" — one paragraph: decision made, evidence written, retrievable via `get_event <event_id>`.
5. Link from README install block as "→ 3-minute quickstart."

---

## P0.1 — Engineering Polish (After Initial MCP Adoption)

Deferred from P0. Tackle after the first developer can complete the quickstart end-to-end.

### Release pipeline hardening

- Run `goreleaser release --snapshot --clean` locally before cutting any release tag; fix all errors first.
- Add smoke test job to `release.yml`: pull published image, run `--version`, assert exit 0.
- Verify Homebrew formula auto-update succeeds on tag push; add failure alert.

### Binary size verification

- Record binary size delta after embedding the bundle (`go build -o /dev/null ./cmd/evidra-mcp && du -sh`); document in PR.
- Set a soft size budget (5 MB increase); revisit if exceeded.

### Quickstart reviewer validation

- Have one reviewer (not the author) complete `docs/quickstart-mcp.md` on a clean machine before the doc is considered stable.
- Fix any broken steps; re-test after each fix.

### CI hardening

- Add `bundle-test.yml` workflow: trigger on `policy/bundles/**` changes; run `opa test policy/bundles/ops-v0.1/ -v`; block merge on failure.
- Add `TestZeroConfigMCPStart` to CI matrix so it runs on every push, not just locally.
- Confirm `go test -race ./...` runs in `ci.yml` (already in Makefile).

### Trust signals

- CI badge (passing) in README header.
- Go Report Card badge (`A` grade) in README header.
- Confirm `LICENSE` file at repo root with correct SPDX identifier.
- Add `SECURITY.md` with vulnerability disclosure contact.
