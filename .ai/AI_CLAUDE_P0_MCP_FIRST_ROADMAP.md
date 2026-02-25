# P0 — MCP-First Rewrite Plan
Generated: 2026-02-25
Source prompt: product_owner_feature_proposal_road_map.md (strategic shift: MCP-first AI safety backend)

---

## Strategic Context

Evidra is repositioned as: **MCP-first AI safety backend for infrastructure agents.**

Gating question for P0:
> Can a developer connect Evidra to an AI agent in under 5 minutes and see it block a dangerous action?

If not — it's P0. Everything else is P1+.

**P0 scope:** zero-config `evidra-mcp` startup, install path for the MCP binary, MCP-first README + demo, 3-minute quickstart.

**Explicitly excluded from P0:** policy library expansion, GitHub Action, coverage tooling, enterprise signals, HTTP transport (→ P1.1), evidence export polish, `list_rules`/`simulate` tools, compliance or SaaS features.

---

## P0 — Critical (Adoption blockers)

> Gating question: Can a developer connect Evidra to an AI agent in under 5 minutes and see it block a dangerous action? If not — it belongs here.

---

### 1. Embed the bundle — zero-config `evidra-mcp` startup

**Why it matters:** `evidra-mcp` fails immediately on any machine that didn't clone the repo. Every install path that doesn't start with `git clone` is broken. Until this is fixed, there is no MCP-first product — there is a local dev tool that happens to have an MCP binary.

**Complexity:** Medium
**Adoption impact:** High
**Owner:** MCP

**Definition of Done:**
- `evidra-mcp` starts with zero flags on a clean machine and is ready to accept MCP tool calls within 2 seconds.
- `docker run ghcr.io/<org>/evidra-mcp` starts without any volume mounts or environment variables.
- `brew install <org>/evidra/evidra-mcp` + launch → server running, embedded bundle loaded.
- stderr emits `using built-in ops-v0.1 bundle (override with --bundle or EVIDRA_BUNDLE_PATH)` when no explicit bundle is set.
- `TestZeroConfigMCPStart` integration test passes: starts the server, calls `validate` with a fixture invocation, asserts a valid decision is returned — no bundle path in env or flags.

**Execution Plan (ordered steps):**
1. Add `//go:embed policy/bundles/ops-v0.1` directive in `cmd/evidra-mcp/main.go`; use `embed.FS`.
2. Write `extractEmbeddedBundle(fs embed.FS, destDir string) error` in `cmd/evidra-mcp/` — binary concern, not a pkg.
3. Extend bundle path resolution in `pkg/config` with a final fallback: call extract into `os.MkdirTemp`, cache path for process lifetime.
4. Print the single-line stderr notice on embedded fallback.
5. Add `TestZeroConfigMCPStart` in `pkg/mcpserver` using an in-process server and a fixture ToolInvocation JSON.
6. Verify binary size increase with `go build -o /dev/null ./cmd/evidra-mcp && du -sh`; document in PR. Accept up to 5 MB increase.
7. Update `Dockerfile.mcp` to `FROM gcr.io/distroless/static`, copy binary only — no bundle volume needed.

---

### 2. Install path: `evidra-mcp` first

**Why it matters:** There is no install path that puts a working `evidra-mcp` binary on a developer's machine in under 60 seconds. Homebrew and Docker are the two paths that signal "this is real software." Without them the project requires cloning, building, and manually resolving the bundle path — that is four steps too many before a developer sees anything work.

**Complexity:** Medium
**Adoption impact:** High
**Owner:** Release

**Definition of Done:**
- `brew install <org>/evidra/evidra-mcp` installs a working `evidra-mcp` binary on macOS with zero flags required to start.
- `docker run ghcr.io/<org>/evidra-mcp:latest` starts the MCP server with embedded bundle, no mounts, no env vars.
- Both artifacts publish automatically within 5 minutes of a `v*` tag push via GoReleaser.
- The published Docker image passes `docker run --rm ghcr.io/<org>/evidra-mcp:latest --version` in CI smoke test.
- README install block links to working Homebrew tap and GHCR image; no placeholder text.

**Execution Plan (ordered steps):**
1. Create `homebrew-evidra` tap repo; add stub formula with placeholder archive URL.
2. Add `brews` block to `.goreleaser.yaml` targeting the tap; set `install: bin.install "evidra-mcp"`. CLI binary is secondary — do not require it.
3. Write `Dockerfile.mcp`: `FROM gcr.io/distroless/static`, copy binary, `ENTRYPOINT ["/evidra-mcp"]`. No bundle volume — relies on embedded bundle from item 1.
4. Add `dockers` block to `.goreleaser.yaml` for `ghcr.io/<org>/evidra-mcp`; tag `{{ .Tag }}` and `latest`.
5. Add GHCR login step to `.github/workflows/release.yml` using `GITHUB_TOKEN`.
6. Run `goreleaser release --snapshot --clean` locally; fix any errors before tagging.
7. Cut `v0.1.0` tag; verify Homebrew formula auto-updates and image appears on GHCR.
8. Add a one-step smoke test job to `release.yml`: pull image, run `--version`, assert exit 0.

---

### 3. MCP-first README and demo

**Why it matters:** The current README leads with architecture. The product is an MCP server — the README should prove that in 90 seconds. A developer evaluating this project needs to see: install one binary, paste one config block, watch an AI agent get blocked. That sequence is not present anywhere in the current docs.

**Complexity:** Low
**Adoption impact:** High
**Owner:** Docs

**Definition of Done:**
- README opens with the product name, the one-line positioning, and a terminal GIF (≤ 3 MB) before any prose.
- The GIF shows: Claude Desktop calling `validate` → PASS, then calling `validate` → FAIL with a rule hint returned.
- Install block appears within the first 20 lines; Homebrew and Docker paths only — no `git clone`.
- A copy-pasteable `claude_desktop_config.json` block for `evidra-mcp` is present, syntactically valid, and tested.
- The word "MCP" appears in the first paragraph. The word "CLI" does not appear before line 40.
- All links resolve. No internal package paths (`pkg/`, `cmd/`) appear in the README.

**Execution Plan (ordered steps):**
1. Confirm `examples/terraform_plan_pass.json` and `examples/terraform_deny_mass_delete.json` both exist and produce the expected PASS/FAIL outcomes against the embedded bundle.
2. Write `demo.tape` for `vhs`: (1) start `evidra-mcp` with no flags, (2) send a PASS `validate` call via Claude Desktop, (3) send a FAIL call, (4) show returned hints in the terminal.
3. Record GIF; verify rendering in GitHub Markdown renderer. Keep under 3 MB.
4. Rewrite README structure: one-line hook → GIF → Homebrew/Docker install → Claude Desktop config block → 3-minute quickstart link → docs link.
5. Write and test the `claude_desktop_config.json` snippet against an actual installed binary; include the exact `args` and `command` fields.
6. Move architecture, package table, and internals to `docs/architecture.md`; link from README footer only.

---

### 4. 3-minute MCP quickstart: connect, call, see a block

**Why it matters:** There is no end-to-end path a developer can follow to go from zero to "AI agent blocked by Evidra" in under 5 minutes. Without this, the product cannot be evaluated. Developers close the tab. This is the single most important adoption artifact the project is missing.

**Complexity:** Low
**Adoption impact:** High
**Owner:** Docs

**Definition of Done:**
- `docs/quickstart-mcp.md` exists and covers the full path: install → configure Claude Desktop → submit a `validate` call → observe a PASS → observe a FAIL with hints — in under 3 minutes of wall-clock time.
- Every command in the quickstart is copy-pasteable and produces the documented output on a clean macOS machine.
- The FAIL scenario uses `examples/terraform_deny_mass_delete.json` and shows the exact hint text returned by the bundle.
- A "What just happened?" section explains the evidence record that was written and how to retrieve it with `get_event`.
- README links to `docs/quickstart-mcp.md` from the install block.
- One reviewer (not the author) completes the quickstart on a clean machine and confirms it works end-to-end before merging.

**Execution Plan (ordered steps):**
1. Write the quickstart doc in `docs/quickstart-mcp.md`. Structure: Prerequisites (1 line: macOS + Claude Desktop) → Install (Homebrew one-liner) → Configure Claude Desktop (paste config block) → Run the PASS scenario → Run the FAIL scenario → Inspect the evidence record.
2. Define the FAIL scenario explicitly: `examples/terraform_deny_mass_delete.json` with a `delete_all` operation against a production namespace. Confirm it triggers `ops.mass_delete`.
3. Include the exact `validate` tool input JSON a developer can paste into Claude's context to trigger the demo, so the quickstart works without a real Terraform plan.
4. Add the "What just happened?" section: one paragraph explaining that a policy decision was made, an evidence record was written, and the record is retrievable via `get_event <event_id>`.
5. Link `docs/quickstart-mcp.md` from the README install block as "→ 3-minute quickstart."
6. Have one reviewer dry-run the full quickstart on a clean machine. Fix any broken steps before shipping.
