You are a release engineer and Go maintainer.

Context:
Project: Evidra
Primary artifact for P0: `evidra-mcp` (offline MCP / stdio).
`evidra-mcp` MUST run with zero flags using an embedded OPA bundle.
HTTP transport is not part of P0.

We already have:
- A working `evidra-mcp` binary
- Embedded bundle fallback when no --bundle and no EVIDRA_BUNDLE_PATH
- A Docker image will exist (P0 step 2)

Now we need:
P0 Step 3 — Homebrew + Release alignment so a developer can install and run in < 2 minutes.

Goal (P0):
`brew install <tap>/evidra-mcp` installs a working `evidra-mcp` that runs with zero flags.

---

TASK:
Produce an implementation-ready plan AND concrete config snippets for:

1) GoReleaser config updates
2) GitHub Actions release workflow updates
3) Homebrew tap repo structure (formula)
4) End-to-end release verification steps (including snapshot)
5) Minimal README install snippet (MCP-first) aligned with the above

---

STRICT REQUIREMENTS:

A) Artifacts
- Must publish `evidra-mcp` for macOS (amd64 + arm64) and Linux (amd64 + arm64).
- Optionally publish `evidra` CLI too, but MCP is primary and must be installed first-class.
- Release artifacts must NOT require repo checkout or bundle path.

B) Homebrew
- Provide a Homebrew tap repo plan: `<org>/homebrew-evidra`
- Provide formula for `evidra-mcp`:
  - installs binary as `evidra-mcp`
  - `test do` runs `evidra-mcp --version` (or equivalent safe check)
- If also installing CLI, do it as a separate formula or as a second binary in same formula — choose the simplest and justify.

C) GoReleaser
- Provide exact `.goreleaser.yaml` blocks:
  - builds section for `evidra-mcp` (and optionally `evidra`)
  - archives naming templates
  - brews section for Homebrew
  - release settings
- Ensure the GoReleaser config supports GitHub Releases and Homebrew formula update.

D) GitHub Actions release workflow
- Provide exact YAML snippet changes:
  - triggers on `v*` tag
  - sets up Go
  - logs into GHCR only if Docker is configured (optional)
  - runs GoReleaser with `GITHUB_TOKEN`
- Include a "release dry run" job:
  - runs `goreleaser release --snapshot --clean`
  - fails if config is invalid

E) Versioning & Flags
- Ensure `evidra-mcp --version` exists and prints semver, commit, date (if missing, specify minimal Go implementation).
- The Homebrew formula must use the release tarball and checksum.

F) Verification
Provide an exact checklist to verify on a clean macOS machine:
1. uninstall old binaries
2. brew tap
3. brew install
4. run `evidra-mcp` (no flags)
5. confirm stderr contains "using built-in ops-v0.1 bundle"
6. confirm tool is executable and prints version

G) Output Format
Return a single Markdown document with these sections:

# P0 Step 3 — Homebrew + Release Alignment

## 1. Decision: packaging strategy
## 2. GoReleaser changes (full snippets)
## 3. Homebrew tap & formula (full file content)
## 4. GitHub Actions release workflow (full snippets)
## 5. Release verification checklist (copy-paste)
## 6. README Install block (MCP-first)

Constraints:
- Startup pragmatism (no enterprise features)
- No speculative future work
- No new heavy dependencies
- Do not mention HTTP transport except as “not in P0”
- Keep the plan minimal and shippable

Be concrete:
- Provide file paths
- Provide code/YAML snippets
- Use placeholders only for <org>, <repo>, etc.
- Assume GitHub as hosting