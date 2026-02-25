# Evidra Product Roadmap
Generated: 2026-02-25

---

# Executive Summary

- **Core is sound, periphery is missing.** The evaluation pipeline (scenario → OPA → evidence) is correctly designed and deterministic. The architecture is clean. What's missing is everything that makes a developer *trust* and *install* a tool: installability, observability, examples that work out of the box, and a README that converts in 90 seconds.
- **The MCP integration is the genuine differentiator** — no other open-source tool positions itself as a safety layer between an AI agent and infrastructure execution. This angle is underexploited in current docs and positioning.
- **Six OPA rules is not a product — but 23 is.** The policy engine is excellent; the policy *library* was a stub. The [Serious Baseline Research](../docs/backlog/OPS_V0_SERIOUS_BASELINE_RESEARCH.md) identified 18 must-have guardrails (1 already implemented) across Terraform, Kubernetes, ArgoCD, S3, and IAM — each backed by CIS controls, tfsec/trivy AVD IDs, or kube-score checks. Adding the remaining 17 brings the bundle to 23 rules covering container escape, mass data exposure, irreversible destruction, account/cluster takeover, and GitOps safety. This is the minimum viable policy library that makes Evidra look serious on day one.
- **The evidence chain is buried.** Hash-linked append-only evidence with chain validation is a strong trust primitive. It is currently mentioned in passing. It should be the headline for every security-conscious DevOps audience.
- **Stdio-only MCP is a real constraint.** Stdio-only MCP limits broader integrations. Offline MCP is sufficient for initial adoption and local agent workflows, but HTTP transport will unlock Copilot/Cursor/Windsurf and remote API-level integrations. This is the next major expansion milestone after P0.
- **Zero discoverability.** No Homebrew formula, no Docker image on GHCR, no GitHub Action, no badge for "installable in 30 seconds." Right now a developer must clone, build, and figure out the bundle path manually.
- **The project is 6–8 weeks of focused work away from being genuinely adoptable** by a DevOps team that uses AI coding assistants.

---

# Priority Matrix

## P0 — Critical (Adoption blockers)

> Can a developer install Evidra, connect it to an AI agent, and see a dangerous action blocked — in under 5 minutes?

**Full detail:** [AI_CLAUDE_P0_MCP_FIRST_ROADMAP.md](./AI_CLAUDE_P0_MCP_FIRST_ROADMAP.md)

| # | Item | Complexity | Adoption impact | Owner |
|---|---|---|---|---|
| 1 | Embed bundle — zero-config `evidra-mcp` startup | Medium | High | MCP |
| 2 | Install path: Homebrew + Docker for `evidra-mcp` | Medium | High | Release |
| 3 | MCP-first README and demo GIF | Low | High | Docs |
| 4 | 3-minute MCP quickstart | Low | High | Docs |

### P0 Exit Criteria

- `evidra-mcp` starts on a clean machine with no flags and no bundle path set.
- `brew install <org>/evidra/evidra-mcp` produces a working binary on macOS.
- `docker run ghcr.io/<org>/evidra-mcp:latest` starts the MCP server with no mounts or env vars.
- README opens with a demo GIF showing `validate` → PASS and `validate` → FAIL with a hint.
- `claude_desktop_config.json` snippet in README is copy-pasteable and connects to a running `evidra-mcp`.
- `docs/quickstart-mcp.md` takes a developer from zero to a blocked action with a visible hint in under 5 minutes.

### P0 Complete When

P0 is complete when a developer unfamiliar with the codebase can install, connect, and observe a blocked action in under 5 minutes without reading any internal documentation.

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

### 0. Serious Baseline Policy Pack — ops-v0.1 expansion to 23 rules

**Why it matters:** Six rules signals a demo skeleton. Twenty-three rules — covering Kubernetes container escape (CIS 5.2.x), Terraform public exposure (tfsec AVD-AWS-*), IAM wildcard policies, S3 encryption/versioning, and ArgoCD operational safety — signals a production-grade tool. Every rule is sourced from industry-standard tools and maps to a documented real-world incident or attack chain.

**Complexity:** Medium (17 new Rego files, params, hints, OPA tests)
**Adoption impact:** **Critical** — policy depth is what makes a security tool credible

**Research document:** [`docs/backlog/OPS_V0_SERIOUS_BASELINE_RESEARCH.md`](../docs/backlog/OPS_V0_SERIOUS_BASELINE_RESEARCH.md)

**Rule inventory (17 new + 6 existing = 23 total):**

| # | Rule ID | Category | Disposition | Source |
|---|---|---|---|---|
| | *Existing rules* | | | |
| 1 | `k8s.protected_namespace` | Kubernetes | DENY | Original |
| 2 | `ops.mass_delete` | Terraform | DENY | Original |
| 3 | `ops.public_exposure` | Terraform | DENY | Original |
| 4 | `ops.unapproved_change` | Ops | DENY | Original |
| 5 | `ops.autonomous_execution` | Ops | WARN | Original |
| 6 | `ops.breakglass_used` | Ops | WARN | Original |
| | *New Kubernetes rules* | | | |
| 7 | `k8s.privileged_container` | Kubernetes | DENY | CIS 5.2.1 |
| 8 | `k8s.host_namespace_escape` | Kubernetes | DENY | CIS 5.2.2/5.2.3/5.2.4 |
| 9 | `k8s.run_as_root` | Kubernetes | DENY | CIS 5.2.6 |
| 10 | `k8s.hostpath_mount` | Kubernetes | DENY | CIS 5.2.13 |
| 11 | `k8s.dangerous_capabilities` | Kubernetes | DENY | CIS 5.2.7/5.2.8 |
| 12 | `k8s.mutable_image_tag` | Kubernetes | WARN | kube-score |
| 13 | `k8s.no_resource_limits` | Kubernetes | WARN | kube-score |
| | *New Terraform rules* | | | |
| 14 | `terraform.sg_open_world` | Terraform | DENY | AVD-AWS-0107 |
| 15 | `terraform.s3_public_access` | Terraform | DENY | AVD-AWS-0086/87/91/93 |
| 16 | `terraform.iam_wildcard_policy` | Terraform | DENY | AVD-AWS-0057 |
| | *New AWS S3 rules* | | | |
| 17 | `aws_s3.no_encryption` | AWS S3 | DENY | AVD-AWS-0088 |
| 18 | `aws_s3.no_versioning_prod` | AWS S3 | DENY | AVD-AWS-0090 |
| | *New AWS IAM rules* | | | |
| 19 | `aws_iam.wildcard_policy` | AWS IAM | DENY | AVD-AWS-0057 |
| 20 | `aws_iam.wildcard_principal` | AWS IAM | DENY | Datadog/Token Security |
| | *New ArgoCD rules* | | | |
| 21 | `argocd.autosync_prod` | ArgoCD | DENY | ArgoCD docs |
| 22 | `argocd.wildcard_destination` | ArgoCD | DENY | ArgoCD docs |
| 23 | `argocd.dangerous_sync_combo` | ArgoCD | DENY | ArgoCD docs |

**Per-rule deliverables:**
- Rego file in `policy/bundles/ops-v0.1/evidra/policy/rules/`
- Params entries in `evidra/data/params/data.json` (with `by_env` for prod-scoped rules)
- Hints entries in `evidra/data/rule_hints/data.json`
- Minimum 2 OPA tests per rule in `policy/bundles/ops-v0.1/tests/`
- `opa test policy/bundles/ops-v0.1/ -v` all green after each wave

---

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

# Quick Wins — 3-Week Sprint

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

**Days 9–12: Serious Baseline Policy Pack (17 new rules)**

Full research and rationale: [`docs/backlog/OPS_V0_SERIOUS_BASELINE_RESEARCH.md`](../docs/backlog/OPS_V0_SERIOUS_BASELINE_RESEARCH.md)

The current bundle has 6 rules. The research identified 18 must-have guardrails (1 already implemented). Adding the remaining 17 brings the total to 23 — enough to be taken seriously on day one.

*Wave 1 — Kubernetes (days 9–10):*
- `k8s.privileged_container` — deny `privileged: true` (CIS 5.2.1)
- `k8s.host_namespace_escape` — deny hostPID/hostIPC/hostNetwork (CIS 5.2.2/5.2.3/5.2.4)
- `k8s.run_as_root` — deny containers running as UID 0 (CIS 5.2.6)
- `k8s.hostpath_mount` — deny hostPath volume mounts (CIS 5.2.13)
- `k8s.dangerous_capabilities` — deny SYS_ADMIN/SYS_PTRACE/NET_RAW (CIS 5.2.7/5.2.8)
- `k8s.mutable_image_tag` — warn on `:latest` or missing tag (kube-score)
- `k8s.no_resource_limits` — warn on missing CPU/memory limits (kube-score)

*Wave 2 — Terraform / IAM / S3 (days 10–11):*
- `terraform.sg_open_world` — deny 0.0.0.0/0 on SSH/RDP (AVD-AWS-0107)
- `terraform.s3_public_access` — deny missing Block Public Access (AVD-AWS-0086/0087/0091/0093)
- `terraform.iam_wildcard_policy` — deny Action:\*/Resource:\* (AVD-AWS-0057)
- `aws_s3.no_encryption` — deny unencrypted buckets (AVD-AWS-0088)
- `aws_s3.no_versioning_prod` — deny versioning disabled in prod (AVD-AWS-0090)
- `aws_iam.wildcard_policy` — deny IAM Action:\*/Resource:\* standalone eval (AVD-AWS-0057)
- `aws_iam.wildcard_principal` — deny Principal:\* in trust policies (Datadog/Token Security research)

*Wave 3 — ArgoCD (days 11–12):*
- `argocd.autosync_prod` — deny automated sync in production
- `argocd.wildcard_destination` — deny wildcard cluster/namespace in AppProjects
- `argocd.dangerous_sync_combo` — deny auto + prune + selfHeal in production

Each rule: Rego file, params entries (with `by_env`), hints entries, two OPA tests minimum.
`opa test policy/bundles/ops-v0.1/ -v` — all green after each wave.

**Day 13: `evidra evidence report`**
- Add `evidra evidence report --format markdown` CLI command.
- Sections: event count, risk distribution, top denied rule IDs, chain status.

**Day 14: HTTP transport on `evidra-mcp`**
- Add `--http-port` flag. Wire `go-sdk`'s HTTP handler.
- Add `--auth-token` bearer auth.
- Document in `docs/mcp-clients-setup.md`.

**Day 15: GitHub Action scaffold**
- Create `evidra-action` repo with `action.yml` and `README.md`.
- Inputs: `input-file`, `bundle-path`, `environment`, `fail-on-deny`.
- Downloads correct binary from GitHub Releases for runner OS/arch.

**Day 16: Release v0.1.0**
- Cut `v0.1.0` git tag.
- GoReleaser publishes binaries, Docker image, Homebrew formula.
- Announce: Go Discord `#show-and-tell`, HN "Show HN", r/devops — lead with the demo GIF.

---

**The brutal truth:** The engineering is ahead of the product. The core works. What is missing is the packaging, the policy library depth, and the narrative that says *"this is the safety layer for AI agents touching real infrastructure"* — and says it in the first 10 seconds on the README. Fix that, ship installable artifacts, add the GitHub Action. That combination is worth more stars than six more months of engine improvements.
