# P0 — MCP-First Plan
Generated: 2026-02-25
Source prompt: product_owner_feature_proposal_road_map.md (strategic shift: MCP-first AI safety backend)

---

## Strategic Context

Evidra is repositioned as: **MCP-first AI safety backend for infrastructure agents.**

Gating question for P0:
> Can a developer install Evidra, connect it to an AI agent, and see a dangerous action blocked — in under 5 minutes?

If not — it's P0. If it does not directly reduce time-to-first-block, it does not belong in P0. Everything else is P0.1+.

**P0 scope:** zero-config `evidra-mcp` startup, install path for the MCP binary, MCP-first README + demo, 3-minute quickstart.

**Explicitly excluded from P0:** policy library expansion, GitHub Action, coverage tooling, enterprise signals, HTTP transport (→ P1), evidence export polish, `list_rules`/`simulate` tools, compliance or SaaS features, CI hardening, binary size verification, release pipeline hardening (→ P0.1).

---

## P0 — Critical (Adoption blockers)

> Can a developer install Evidra, connect it to an AI agent, and see a dangerous action blocked — in under 5 minutes?

If it does not directly reduce time-to-first-block, it does not belong in P0.

---

### 1. Embed the bundle — zero-config `evidra-mcp` startup

**Why it matters:** `evidra-mcp` fails on any machine that didn't clone the repo. Every install path is broken until the bundle ships inside the binary.

**Complexity:** Medium
**Adoption impact:** High
**Owner:** MCP

**Definition of Done:**
- `evidra-mcp` starts on a clean machine with no flags and accepts a `validate` tool call.
- `docker run ghcr.io/<org>/evidra-mcp` starts and serves `validate` with no mounts or env vars.

**Execution Plan:**
1. Add `//go:embed policy/bundles/ops-v0.1` in `cmd/evidra-mcp/main.go`; use `embed.FS`.
2. Write `extractEmbeddedBundle(fs embed.FS, destDir string) error` in `cmd/evidra-mcp/`.
3. Extend bundle path resolution in `pkg/config`: final fallback extracts into `os.MkdirTemp`, cached for process lifetime.
4. Print `using built-in ops-v0.1 bundle (override with --bundle or EVIDRA_BUNDLE_PATH)` to stderr on fallback.
5. Update `Dockerfile.mcp`: `FROM gcr.io/distroless/static`, copy binary only.

---

### 2. Install path: `evidra-mcp` first (Homebrew + Docker)

**Why it matters:** Without Homebrew or Docker, every install requires cloning and building. A developer needs one command to get a running MCP server.

**Complexity:** Medium
**Adoption impact:** High
**Owner:** Release

**Definition of Done:**
- `brew install <org>/evidra/evidra-mcp` produces a binary that starts with no flags.
- `docker run ghcr.io/<org>/evidra-mcp:latest` starts the MCP server with no mounts or env vars.
- README install block links to the working tap and image with no placeholder text.

**Execution Plan:**
1. Create `homebrew-evidra` tap repo; add formula pointing at GoReleaser archive.
2. Add `brews` block to `.goreleaser.yaml`; `install: bin.install "evidra-mcp"`.
3. Write `Dockerfile.mcp`: `FROM gcr.io/distroless/static`, copy binary, `ENTRYPOINT ["/evidra-mcp"]`.
4. Add `dockers` block to `.goreleaser.yaml`; tag `{{ .Tag }}` and `latest`.
5. Add GHCR login to `.github/workflows/release.yml` using `GITHUB_TOKEN`.
6. Cut `v0.1.0` tag; verify tap formula and GHCR image are live.

---

### 3. MCP-first README and demo

**Why it matters:** The README leads with architecture. A developer must see install → config → blocked action in under 90 seconds of reading — before any explanation.

**Complexity:** Low
**Adoption impact:** High
**Owner:** Docs

**Definition of Done:**
- README opens with a terminal GIF before any prose.
- GIF shows: `validate` → PASS, then `validate` → FAIL with a hint.
- Install block appears within the first 20 lines; Homebrew and Docker only.
- A `claude_desktop_config.json` snippet is present, copy-pasteable, and syntactically valid.

**Execution Plan:**
1. Confirm `examples/terraform_plan_pass.json` and `examples/terraform_deny_mass_delete.json` produce PASS and FAIL.
2. Write `demo.tape` for `vhs`: start `evidra-mcp`, PASS call, FAIL call, hints visible.
3. Record GIF.
4. Rewrite README: GIF → install → `claude_desktop_config.json` → quickstart link.
5. Move architecture and package internals to `docs/architecture.md`; link from footer.

---

### 4. 3-minute MCP quickstart: connect, call, see a block

**Why it matters:** There is no path from zero to "blocked by Evidra" today. Without it the product cannot be evaluated. Developers close the tab.

**Complexity:** Low
**Adoption impact:** High
**Owner:** Docs

**Definition of Done:**
- `docs/quickstart-mcp.md` takes a developer from install to a blocked action with a visible hint in under 3 minutes.
- Every command is copy-pasteable and works on a clean macOS machine.
- The FAIL scenario shows the exact hint text returned by the bundle.

**Execution Plan:**
1. Write `docs/quickstart-mcp.md`: Install → Configure Claude Desktop → PASS call → FAIL call → see hint.
2. Confirm `examples/terraform_deny_mass_delete.json` triggers `ops.mass_delete`.
3. Include the exact `validate` input JSON — quickstart must work without a real Terraform plan.
4. Add "What just happened?" — one paragraph: policy fired, evidence written, retrievable via `get_event <event_id>`.
5. Link from README install block as "→ 3-minute quickstart."

---

## P0.1 — Engineering Polish (After Initial MCP Adoption)

> Important — but not required for first adoption. Tackle after the first developer completes the quickstart end-to-end.

### Release pipeline hardening

- Run `goreleaser release --snapshot --clean` locally before cutting any release tag.
- Add smoke test job to `release.yml`: pull published image, run `--version`, assert exit 0.
- Verify Homebrew formula auto-update succeeds on tag push; add failure alert.

### Binary size verification

- Record binary size delta after embedding the bundle; document in PR.
- Set a soft size budget (5 MB increase); revisit if exceeded.

### Quickstart reviewer validation

- Have one reviewer (not the author) complete `docs/quickstart-mcp.md` on a clean machine.
- Fix any broken steps; re-test after each fix.

### CI hardening

- Add `bundle-test.yml` workflow: trigger on `policy/bundles/**`; run `opa test`; block merge on failure.
- Add `TestZeroConfigMCPStart` to CI matrix.
- Confirm `go test -race ./...` runs in `ci.yml`.

### Trust signals

- CI badge in README header.
- Go Report Card badge in README header.
- Confirm `LICENSE` at repo root with correct SPDX identifier.
- Add `SECURITY.md` with vulnerability disclosure contact.
