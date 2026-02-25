# Evidra Product Roadmap
Generated: 2026-02-25

---

# Executive Summary

- **Core is sound, periphery is missing.** The evaluation pipeline (scenario → OPA → evidence) is correctly designed and deterministic. The architecture is clean. What's missing is everything that makes a developer *trust* and *install* a tool: installability, observability, examples that work out of the box, and a README that converts in 90 seconds.
- **The MCP integration is the genuine differentiator** — no other open-source tool positions itself as a safety layer between an AI agent and infrastructure execution. This angle is underexploited in current docs and positioning.
- **Six OPA rules is not a product.** The policy engine is excellent; the policy *library* is a stub. Developers who clone and see six rules for `kube-system` and `prod` namespace will assume the project is a demo skeleton.
- **The evidence chain is buried.** Hash-linked append-only evidence with chain validation is a strong trust primitive. It is currently mentioned in passing. It should be the headline for every security-conscious DevOps audience.
- **Stdio-only MCP is a real constraint.** Every MCP integration other than Claude Desktop and process-local agents needs HTTP. This is the single biggest blocker for Copilot/Cursor/Windsurf/API-level integrations.
- **Zero discoverability.** No Homebrew formula, no Docker image on GHCR, no GitHub Action, no badge for "installable in 30 seconds." Right now a developer must clone, build, and figure out the bundle path manually.
- **The project is 6–8 weeks of focused work away from being genuinely adoptable** by a DevOps team that uses AI coding assistants.

---

# Priority Matrix

## P0 — Critical (Adoption blockers)

> Can a developer connect Evidra to an AI agent in under 5 minutes and see it block a dangerous action?

**Full detail:** [AI_CLAUDE_P0_MCP_FIRST_ROADMAP.md](./AI_CLAUDE_P0_MCP_FIRST_ROADMAP.md)

| # | Item | Complexity | Adoption impact | Owner |
|---|---|---|---|---|
| 1 | Embed bundle — zero-config `evidra-mcp` startup | Medium | High | MCP |
| 2 | Install path: Homebrew + Docker for `evidra-mcp` | Medium | High | Release |
| 3 | MCP-first README and demo GIF | Low | High | Docs |
| 4 | 3-minute MCP quickstart | Low | High | Docs |

---

## P0.1 — Engineering Polish (After Initial MCP Adoption)

Deferred from P0. Full detail in [AI_CLAUDE_P0_MCP_FIRST_ROADMAP.md](./AI_CLAUDE_P0_MCP_FIRST_ROADMAP.md).

| Group | Items |
|---|---|
| Release pipeline | goreleaser snapshot dry-run, smoke test job, Homebrew auto-update alert |
| Binary size | size delta recorded in PR, 5 MB soft budget |
| Quickstart validation | reviewer dry-run on clean machine |
| CI hardening | `bundle-test.yml` workflow, `TestZeroConfigMCPStart` in matrix, race detector in `ci.yml` |
| Trust signals | CI badge, Go Report Card badge, `LICENSE`, `SECURITY.md` |

---

## P1 — High Value (Adoption accelerators)

### 1. Add HTTP transport to the MCP server

**Why it matters:** Claude Desktop and process-local agents support stdio. Everything else — Copilot extensions, Cursor remote workspaces, custom agent frameworks, CI pipelines calling the MCP server over the network — needs HTTP. Stdio-only is a fundamental constraint that prevents the "AI backend" positioning from being real for most integration patterns.

See [Hosted API MVP Architecture](../docs/architecture_hosted-api-mvp.md) for the full HTTP transport and API design.

**Complexity:** Medium
**Adoption impact:** High

**Steps:**
- Add `--http-port <port>` flag to `evidra-mcp`.
- The `go-sdk` MCP library (v1.3.1) supports HTTP transport via `server.NewHTTPHandler()` — wire this up.
- Add optional bearer token auth: `--auth-token` flag; validate `Authorization: Bearer <token>` header when set.
- Document the HTTP transport in `docs/mcp-clients-setup.md` with examples for Cursor, Windsurf, and direct `curl` testing.
- Add a health endpoint `GET /health` returning `{"status":"ok","version":"...","mode":"enforce|observe"}`.

---

### 2. Structured logging (slog)

**Why it matters:** Any DevOps team deploying `evidra-mcp` as a sidecar or daemon needs structured logs to route to their SIEM, Datadog, or CloudWatch. Right now the server emits nothing useful. Operators cannot tell what's being evaluated, what's being denied, or whether the server is healthy — without reading raw evidence files.

**Complexity:** Low
**Adoption impact:** Medium

**Steps:**
- Add `log/slog` (stdlib, no new dependency) throughout `pkg/mcpserver` and `pkg/validate`.
- Log fields per evaluation: `event_id`, `tool`, `operation`, `environment`, `allow`, `risk_level`, `rule_ids`, `duration_ms`.
- Add `--log-level` flag (`debug`|`info`|`warn`|`error`, default `info`) and `--log-format` (`text`|`json`, default `text`).
- In JSON format, every line is a structured log record — trivially parseable by any log aggregator.
- Never log `params` or `payload` content at `info` level (may contain secrets); log at `debug` only.

---

### 3. MCP tool: `list_rules`

**Why it matters:** AI agents calling `validate` need to know what rules exist before they can reason about what tags to add or what changes to make. Right now there is no way for an agent to discover `k8s.protected_namespace` or `ops.mass_delete` without reading raw Rego files. Adding `list_rules` makes the MCP server a self-describing policy backend.

**Complexity:** Low
**Adoption impact:** Medium

**Steps:**
- Add `list_rules` MCP tool to `pkg/mcpserver/server.go`.
- Output: array of `{rule_id, decision_type, description, hints, params}` objects — derived from the policy bundle's rule_hints and params data files at startup.
- Since rule metadata is static per bundle load, build the index once at startup and cache it.
- This enables an AI agent to say *"before I do X, let me check if any rules apply"* — a proactive safety pattern.

---

### 4. Policy simulation in the MCP server (`simulate` tool)

**Why it matters:** The CLI has `evidra policy sim` but the MCP server has no equivalent. An AI agent that wants to test *"would this action be denied?"* before actually performing it cannot do so via MCP. This means agents using MCP are flying blind until `validate` returns a denial — at which point they've already recorded a failed attempt.

**Complexity:** Low
**Adoption impact:** Medium

**Steps:**
- Add `simulate` MCP tool: same input as `validate`, but `skip_evidence: true` — evaluates policy and returns the decision without writing to the evidence store.
- Rename the existing validate logic to accept a `record bool` parameter internally.
- Document the intended agent pattern: call `simulate` first to check feasibility, then call `validate` to record the actual attempt.

---

## P2 — Strategic (Differentiation & AI positioning)

### 1. Pre-built agent profiles ("agent personas")

**Why it matters:** The most common AI agent use cases are: Terraform apply bot, kubectl ops bot, PR review agent. Each has a predictable risk profile. Shipping named configuration profiles like `--profile terraform-agent` or `--profile k8s-ops` that pre-load the right rules, params, and default environment makes adoption a 5-minute exercise instead of a policy-authoring project.

**Complexity:** Medium
**Adoption impact:** High

**Steps:**
- Define three initial profiles as named bundle configurations in the repo: `terraform-agent`, `k8s-ops`, `general`.
- Each profile is a `data.json` override file + profile-specific rule enable/disable list.
- Add `--profile` flag to both binaries that selects the profile, overriding default params.
- Profiles ship embedded in the binary alongside the base bundle.
- Document each profile with: "intended agent type", "active rules", "recommended risk tags".

---

### 2. Evidence-linked decision resource for AI context injection

**Why it matters:** A killer MCP use case is: an AI agent calls `validate`, gets back an `event_id`, and then references that event in its reasoning trace — *"I verified this action via Evidra (event: evt-1234) before executing."* This creates a human-auditable thread linking AI reasoning to policy outcomes. The infrastructure for this exists (`evidra://event/{id}` resource), but it is not surfaced in agent prompts or documented as an integration pattern.

**Complexity:** Low
**Adoption impact:** High

**Steps:**
- Document the "evidence-linked agent" pattern in `docs/mcp-clients-setup.md`: how to inject the `evidra://event/{id}` resource into agent context after a validate call.
- Add a `system_prompt_snippet` field to the MCP server's `validate` response that clients can optionally inject: `"Action validated by Evidra. Evidence: evt-1234. Risk: low."`.
- Create a reference Claude agent prompt that demonstrates using Evidra as a pre-flight check before Terraform apply.

---

### 3. `evidra evidence report` — human-readable compliance report

**Why it matters:** Security teams and engineering managers do not read JSONL evidence files. They read PDFs and HTML dashboards. A single command that produces a readable summary of recent AI agent activity — what was validated, what was denied, what risk levels were recorded — is the difference between "dev tool" and "governance tool." Governance tools get budget.

**Complexity:** Medium
**Adoption impact:** Medium

**Steps:**
- Add `evidra evidence report --since <duration> --format html|markdown|text` to the CLI.
- Report sections: summary (total events, pass/fail/observe, risk distribution), denied events (rule IDs, hints, timestamps), top actors, evidence chain integrity status.
- HTML output uses a self-contained template (no external CDN dependencies — works air-gapped).
- Markdown output is suitable for pasting into GitHub PRs or Confluence pages.

---

### 4. OpenTelemetry traces from the MCP server

**Why it matters:** In any realistic deployment, `evidra-mcp` runs as a sidecar alongside an AI agent process. DevOps engineers deploying this expect distributed traces — they need to correlate an agent's tool call with an Evidra validation event in Datadog, Honeycomb, or Jaeger. OPA already brings `opentelemetry` as a transitive dependency. The plumbing is 90% there.

**Complexity:** Medium
**Adoption impact:** Medium

**Steps:**
- Wire `otel/trace` spans around `EvaluateScenario()` in `pkg/validate`.
- Span attributes: `evidra.event_id`, `evidra.allow`, `evidra.risk_level`, `evidra.rule_ids`, `evidra.environment`.
- `--otel-endpoint` flag on `evidra-mcp` for OTLP gRPC export (disabled by default).
- When disabled, a no-op tracer is used — zero overhead.

---

### 5. Policy bundle registry (read-only pull model)

**Why it matters:** Right now operators ship the policy bundle alongside the binary or mount it as a volume. This means updating policy requires redeploying. A pull model — `evidra-mcp --bundle-url https://bundles.example.com/ops-v0.1.tar.gz` — lets teams update policy centrally without restarting every agent. It also opens the door to a hosted policy registry as a commercial offering.

**Complexity:** High
**Adoption impact:** Medium

**Steps:**
- Add `--bundle-url <url>` flag. On startup, download, verify checksum (SHA256 in a `.sha256` sidecar file), extract to a temp dir, and load.
- Poll for updates on a configurable interval (`--bundle-refresh <duration>`, default disabled).
- On refresh: download new bundle, compare checksum to current, reload in-place if changed.
- Publish the ops-v0.1 bundle to GitHub Releases as a `tar.gz` artifact (GoReleaser `extra_files` block).
- This enables `--bundle-url https://github.com/<org>/evidra/releases/latest/download/ops-v0.1.tar.gz`.

---

## P3 — Long-term (Scale / Enterprise / Monetization)

### 1. Cryptographic evidence signing (ed25519)

Sign each `EvidenceRecord` with a keypair managed by the operator. Verifiable by `evidra evidence verify --pubkey`. Enables compliance use cases where the evidence chain must be externally auditable without trusting the host that created it.

### 2. Policy-as-code marketplace / public registry

A `evidra policy pull <org/bundle-name>` command that fetches community-contributed policy bundles from a central registry. The hosted registry is the commercial foothold: free for public bundles, paid for private org bundles with RBAC and signing.

### 3. RBAC and multi-tenant evidence stores

Evidence namespaced by team/project. `evidra-mcp` running as a shared service with per-caller authentication. Required for enterprise deployments where multiple AI agent instances share infrastructure.

### 4. Pre-built integrations: ArgoCD, Atlantis, Spacelift

Native operator/plugin that intercepts `terraform plan` output before `apply` and calls Evidra. These are the platforms where DevOps engineers actually run Terraform — not the CLI.

### 5. Attestation format compatibility (in-toto / SLSA)

Map `EvidenceRecord` to the in-toto `Link` format. This makes Evidra evidence consumable by SLSA compliance tooling, attestation verification chains, and supply chain security frameworks — a strong enterprise procurement argument.

---

# AI Backend Strategy

## Required MCP tools (current + additions)

| Tool | Status | Priority |
|---|---|---|
| `validate` | shipped | — |
| `get_event` | shipped | — |
| `simulate` | missing | P1 |
| `list_rules` | missing | P1 |
| `explain_decision` | missing | P2 |
| `get_hint` | missing | P2 |
| `list_events` | missing | P2 |

`explain_decision` takes an `event_id` and returns a structured natural-language explanation of what fired, why, and what the agent should do differently — purpose-built for injection back into the agent's context window.

`list_events` with `--since`, `--actor`, `--risk_level` filters lets a supervising agent review recent history before taking a new action.

## Required APIs

The MCP tools are enough for agent-to-Evidra communication. What is missing is a minimal **admin HTTP API** for operators (not agents):

- `GET /api/v1/health` — liveness
- `GET /api/v1/status` — bundle revision, mode, evidence chain stats
- `GET /api/v1/events?since=<ts>&risk_level=high` — event query
- `GET /api/v1/events/{id}` — single event
- `POST /api/v1/events/{id}/verify` — chain integrity for one event

This API is distinct from the MCP transport and is used by dashboards, CI checks, and alerting systems. It does not need to be a priority today, but it is a prerequisite for any enterprise deployment.

## Required policy UX improvements

- **Rule discovery**: AI agents need `list_rules` to understand what guardrails apply before acting. Without it, agents learn policies reactively (by getting denied) rather than proactively.
- **Hint quality**: Hints must be written for AI consumption, not just human consumption. `"Add risk_tag: breakglass"` is fine for a human; an AI agent needs `"To bypass this rule, include \"breakglass\" in the risk_tags array of the affected action object."` Consider a `hints_for_agent` field parallel to the human `hints` field.
- **Param introspection**: Add a `list_params` MCP tool or include current resolved param values in the `list_rules` response. An agent should know the current `max_deletes` threshold before it submits a delete operation.
- **Observe mode semantics**: Document and enforce that in observe mode, warnings accumulate — an AI agent that calls `validate` 100 times in observe mode is building an evidence record even though it is never blocked. This is important for audit use cases.

## Required evidence/report improvements

- **Agent activity timeline**: Add a secondary index (lightweight SQLite or flat sorted file) that enables fast `list_events?actor=agent&since=1h` queries without scanning all JSONL segments.
- **Risk trend reports**: A command that produces a risk level histogram over time — useful for showing a security team "AI agent activity has 5% high-risk events this week, down from 8% last week."
- **Per-rule firing counts**: Aggregated from the evidence chain — which rules fire most often? This drives policy tuning and is the primary feedback loop for operators.

---

# Packaging & Distribution Strategy

## Releases

GoReleaser is already configured. Additions needed:

```yaml
# .goreleaser.yaml additions

archives:
  - id: evidra-mcp
    builds: [evidra-mcp]
    name_template: "evidra-mcp_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

brews:
  - name: evidra
    repository:
      owner: <org>
      name: homebrew-evidra
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    homepage: "https://github.com/<org>/evidra"
    description: "Policy validation and evidence chain for AI-driven infrastructure changes"
    install: |
      bin.install "evidra"
      bin.install "evidra-mcp"

dockers:
  - image_templates:
      - "ghcr.io/<org>/evidra-mcp:{{ .Tag }}"
      - "ghcr.io/<org>/evidra-mcp:latest"
    dockerfile: Dockerfile.mcp
    extra_files:
      - policy/bundles/ops-v0.1/
```

Minimal `Dockerfile.mcp`:

```dockerfile
FROM gcr.io/distroless/static
COPY evidra-mcp /evidra-mcp
COPY policy/bundles/ops-v0.1/ /bundles/ops-v0.1/
ENTRYPOINT ["/evidra-mcp", "--bundle", "/bundles/ops-v0.1"]
```

## GitHub Actions

Three workflows needed:

1. **`ci.yml`** (exists) — test + lint on every push/PR. Add: `opa test` step, coverage upload to Codecov.
2. **`release.yml`** (exists) — GoReleaser on `v*` tags. Add: Docker push to GHCR, Homebrew formula update.
3. **`bundle-test.yml`** (new) — runs on any change to `policy/bundles/**`. Runs `opa test` in strict mode. Blocks merge if OPA tests fail.

## Homebrew

Publish a `homebrew-evidra` tap repo. Formula installs both `evidra` and `evidra-mcp`. This is the install path for macOS developers and is the fastest path to "I trust this is real software."

## Docker

Two images on GHCR:
- `ghcr.io/<org>/evidra-mcp:latest` — the MCP server, ready to mount as a sidecar.
- `ghcr.io/<org>/evidra:latest` — the CLI, for use in CI pipelines.

## Example repos

**`evidra-examples`** — 20+ scenario JSON files with comments. Includes a `Makefile` that runs all examples and shows expected output.

**`evidra-terraform-demo`** — A minimal Terraform project (S3 bucket, EC2 instance, security group) with a GitHub Actions workflow that runs `evidra validate` on every `terraform plan` output.

## Demo scenario

The demo must show what is genuinely new — not just policy validation but **AI agent + policy + evidence chain**:

1. Claude Desktop with `evidra-mcp` as an MCP server.
2. User asks Claude: *"Delete all pods in the kube-system namespace."*
3. Claude calls `validate` → Evidra returns `allow: false`, `hints: ["Add risk_tag: breakglass", "Or apply changes outside kube-system"]`.
4. Claude responds: *"I can't do that — Evidra policy k8s.protected_namespace blocks changes to kube-system."*
5. User says: *"Fine, delete the pods in the default namespace."*
6. Claude calls `validate` → PASS, evidence recorded.
7. `evidra evidence verify` confirms the chain is intact.

Record as asciinema or screen recording. Lead the README with this.

---

# Quick Wins — 2-Week Sprint

Ordered by dependency and impact.

**Days 1–2: Zero-config binary**
- Embed ops-v0.1 bundle with `//go:embed` in both `cmd/` binaries.
- Implement auto-extract fallback in config resolution.
- Test `go install` + `evidra validate examples/terraform_plan_pass.json` with no other setup.

**Days 3–4: README rewrite + demo GIF**
- Rewrite README with the structure from P0 item 1.
- Record a terminal GIF with `vhs`: pass case, deny case, evidence verify.
- Add copy-pasteable Claude Desktop MCP config block.

**Day 5: Structured logging**
- Add `log/slog` to `pkg/mcpserver` and `pkg/validate`. No new dependency.
- `--log-format json` flag for operator deployments.

**Days 6–7: `list_rules` + `simulate` MCP tools**
- Build rule index at startup from bundle hints and params data.
- Add both tools to `pkg/mcpserver/server.go`.
- Update `docs/mcp-clients-setup.md`.

**Day 8: GoReleaser — Docker + Homebrew wiring**
- Add `dockers` block to `.goreleaser.yaml`.
- Create `Dockerfile.mcp`.
- Create `homebrew-evidra` tap repo and wire GoReleaser `brews` block.
- Test with `goreleaser release --snapshot --clean`.

**Days 9–10: Five new OPA rules**
- `k8s.rbac_cluster_admin`, `k8s.privileged_container`, `tf.iam_wildcard_policy`, `ops.no_dry_run`, `tf.destroy_production`.
- Each rule: Rego file, params/hints entries, two OPA tests, POLICY_CATALOG rule card.
- `opa test policy/bundles/ops-v0.1/ -v` — all green.

**Day 11: `evidra evidence report`**
- Add `evidra evidence report --format markdown` CLI command.
- Sections: event count, risk distribution, top denied rule IDs, chain status.

**Day 12: HTTP transport on `evidra-mcp`**
- Add `--http-port` flag. Wire `go-sdk`'s HTTP handler.
- Add `--auth-token` bearer auth.
- Document in `docs/mcp-clients-setup.md`.

**Day 13: GitHub Action scaffold**
- Create `evidra-action` repo with `action.yml` and `README.md`.
- Inputs: `input-file`, `bundle-path`, `environment`, `fail-on-deny`.
- Downloads correct binary from GitHub Releases for runner OS/arch.

**Day 14: Release v0.1.0**
- Cut `v0.1.0` git tag.
- GoReleaser publishes binaries, Docker image, Homebrew formula.
- Announce: Go Discord `#show-and-tell`, HN "Show HN", r/devops — lead with the demo GIF.

---

**The brutal truth:** The engineering is ahead of the product. The core works. What is missing is the packaging, the policy library depth, and the narrative that says *"this is the safety layer for AI agents touching real infrastructure"* — and says it in the first 10 seconds on the README. Fix that, ship installable artifacts, add the GitHub Action. That combination is worth more stars than six more months of engine improvements.
