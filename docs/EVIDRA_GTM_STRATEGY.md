# Evidra — Go-To-Market Strategy

**Strategic frame:** You're not building a security product. You're building a **social permission layer for AI infrastructure automation.** This gives DevOps the ability to say: "We're safe."

**One growth lever:** MCP. Nothing else until MCP works.

---

## Three Phases, Hard Boundaries

```
Phase 1: Become the MCP kill-switch          0-60 days
   Only 5 things matter. Ship them. Get retention.

Phase 2: DevOps legitimacy                   60-120 days
   CI, evidence reporting, case studies.
   Only start when Phase 1 retention > 30%.

Phase 3: Enterprise pull                     After demand
   Only when someone with a budget asks.
   Not before.
```

---

## Phase 1: Become the MCP Kill-Switch (Days 0-60)

Five deliverables. Nothing else. If you're working on anything not on this list, stop.

### 1. Hosted MCP Endpoint — Bulletproof

The hosted endpoint IS the product for v0.1. Not the binary. Not the CLI. The endpoint.

```json
{"mcpServers": {"evidra": {"url": "https://evidra.samebits.com/mcp"}}}
```

Requirements:
- Zero signup. Zero API key. Zero config.
- Rate limited (per-IP, generous for individual use).
- p99 latency < 500ms.
- Uptime monitoring with alerts. If this goes down during HN spike, launch is dead.
- Test under 500 concurrent connections before announcing.

**If the hosted endpoint isn't reliable, don't launch.** People will try it once, get a timeout, and never come back.

### 2. Hero Screenshot

One image that tells the entire story. Claude Code conversation:

```
User: Delete all pods in kube-system
Claude: I need to validate this with Evidra first.
        [calls validate tool]
        This operation was DENIED.
        
        Rule: k8s.protected_namespace
        Risk: high
        Hint: Add risk_tag: breakglass, or apply changes 
              outside kube-system
        
        I won't proceed with this deletion.
```

This screenshot goes:
- At the top of the README (before any text)
- In the HN post
- In the tweet
- In the blog post

No screenshot = no launch. This is the product.

### 3. Warn-Only Mode

This is the adoption unlock you haven't built yet. Critical.

Right now Evidra has two modes: `ops` (full protection) and `baseline` (kill-switch only). Both DENY. There's no mode where Evidra says "this would have been blocked, but I'm letting it through."

**Why this matters:** A new user who gets blocked on something legitimate in the first 5 minutes uninstalls. A user who sees "WARNING: this would have been blocked by k8s.privileged_container" learns what the tool does without friction.

```bash
EVIDRA_MODE=warn    # log + warn, never block
EVIDRA_MODE=enforce # default — block on deny
```

Implementation: In the decision path, when mode=warn, return `allow: true` but include full rule hits, reasons, and hints in the response. The AI agent sees the warning and can mention it to the user, but proceeds with the operation.

**The adoption path becomes:**
1. User installs with default (enforce) mode
2. Gets blocked on something they consider fine
3. Instead of uninstalling, switches to `EVIDRA_MODE=warn`
4. Runs for a week, sees what would have been blocked
5. Understands the rules, switches back to enforce
6. Now they trust it

Without warn mode, step 2 → uninstall. With warn mode, step 2 → learning.

### 4. README Restructure

Current README is written for architects. Rewrite for someone who has 30 seconds.

```markdown
# Evidra

Infrastructure kill-switch for AI agents.

[HERO SCREENSHOT HERE]

## Try It Now (30 seconds)

Add to ~/.claude/settings.json:

    {"mcpServers": {"evidra": {"url": "https://evidra.samebits.com/mcp"}}}

Restart Claude Code. Type:

    "Delete all pods in kube-system."

Claude will check with Evidra. Evidra will say no.

## Test These Prompts

| Try this | What happens |
|----------|-------------|
| "Delete all pods in kube-system" | DENIED — protected namespace |
| "Create a public S3 bucket" | DENIED — public exposure |
| "Run a privileged container" | DENIED — privilege escalation |
| "Open SSH to 0.0.0.0/0" | DENIED — world-open security group |
| "Deploy nginx to default namespace" | ALLOWED — safe operation |

## Want It Offline?

    brew install vitas/tap/evidra-mcp

---

Everything below is for people who want to understand how it works.
```

**Above the fold: zero mention of OPA, evidence chains, Ed25519, hash-linked logs, policy bundles.** That's all below the fold for people who scroll.

### 5. Five Curated Prompts

The five prompts in the table above are not arbitrary. Each demonstrates a different capability:

| Prompt | Rule | What it proves |
|--------|------|----------------|
| Delete pods in kube-system | `k8s.protected_namespace` | Namespace protection |
| Public S3 bucket | `terraform.s3_public_access` | Cloud misconfiguration |
| Privileged container | `k8s.privileged_container` | Container security |
| SSH to 0.0.0.0/0 | `terraform.sg_open_world` | Network security |
| Deploy nginx to default | (no rule fires) | Not everything is blocked |

The fifth prompt is the most important. It proves Evidra isn't just "deny everything." It's the moment the user thinks "ok, this is smart."

**Test all five prompts against the hosted endpoint with Claude Code before launch.** If any behaves unexpectedly, fix the rule or swap the prompt.

---

## Phase 2: DevOps Legitimacy (Days 60-120)

Only start when Phase 1 shows retention > 30% (users still have Evidra in settings.json after 2 weeks). If retention is low, fix Phase 1 first — no amount of CI integration saves a bad core experience.

Four deliverables.

### 1. GitHub Actions Example

```yaml
# .github/workflows/terraform-validate.yml
- name: Validate Terraform Plan
  run: |
    terraform show -json tfplan | \
      evidra validate --tool terraform --op apply
```

Ship as `examples/github-actions/`. This is the bridge from "MCP toy" to "CI tool."

### 2. `evidra evidence last`

```bash
evidra evidence last        # most recent decision, human-readable
evidra evidence last 5      # last 5
evidra evidence violations  # just the denies
```

30 lines of code. Huge usability improvement for anyone who wants to understand what Evidra blocked.

### 3. One-Pager for "Send to Your Boss"

A PDF (not a README) that the DevOps engineer forwards:
- What: kill-switch for AI agent infrastructure operations
- Why: AI agents run kubectl/terraform without review
- How: validates before execution, logs everything
- Risk: pre-execution only, worst case = passes something (same as without Evidra)
- License: Apache 2.0, no SaaS, no telemetry, runs locally
- Try it: staging, 10 minutes, zero infra changes

### 4. Blog Post

"I Let Claude Code Manage My Kubernetes Cluster for 7 Days. Here's Everything It Tried to Do."

Narrative structure:
- Day 1: Claude suggests deleting pods, Evidra blocks
- Day 3: Claude tries privileged container, Evidra blocks
- Day 5: Claude learns to ask for breakglass when needed
- Day 7: Evidence log shows 23 blocks, 0 false positives

Tone: entertaining first, technical second. X/HN optimized.

---

## Phase 3: Enterprise Pull (Only After Demand)

Do NOT build any of this unless someone with a budget is asking. "Interesting" conversations don't count. A signed pilot agreement or a PO counts.

| Feature | Build trigger |
|---------|--------------|
| Custom bundle overlay (`EVIDRA_CUSTOM_BUNDLE_PATH`) | 5+ GitHub issues requesting custom rules |
| SIEM forwarder (Splunk/Datadog) | Enterprise pilot conversation |
| Evidence dashboard | 10+ users with >1000 evidence records |
| SaaS / team plans | 50+ active hosted endpoint users |
| Pulumi / CDK adapter | Community PR or 10+ issues |
| SSO / RBAC | Enterprise contract negotiation |

---

## What NOT to Build (Phase 1 Discipline)

### No `EVIDRA_DISABLED_RULES`

The previous assessment recommended `EVIDRA_DISABLED_RULES` as a launch requirement. On reflection, this is wrong. Here's why:

If you let users disable individual rules, the path of least resistance becomes "disable whatever blocks me." Within a month, teams have 5 rules disabled and the tool provides no value. At enterprise rollout, the disabled rules become the standard and nobody remembers why they were disabled.

**Instead, use the existing architecture:**

1. **Profiles** — already built. `EVIDRA_PROFILE=baseline` gives kill-switch only, no opinion rules. `EVIDRA_PROFILE=ops` gives full protection. This is the coarse-grained escape hatch.

2. **Environment-aware behavior** — already built. `by_env` overrides in `data.json` let you configure per-environment. Production strict, staging relaxed.

3. **Breakglass** — already built. The AI agent can tag an operation with `risk_tag: breakglass` to override specific rules. This is tracked in evidence. It's an override with accountability, not a silent disable.

4. **Warn mode** (new in P0) — `EVIDRA_MODE=warn` lets everything through but logs what would have been blocked. This is the "I'm still learning" phase.

The progression:
```
warn mode → baseline profile → ops profile → ops with breakglass as needed
```

Each step is a deliberate security posture increase. No step involves silently disabling rules.

### No CI Mode Emphasis at Launch

CI works. It's already built. But it's not the entry point. The README mentions it below the fold. The GitHub Actions example ships in P1, not P0.

Nobody puts a new tool in CI on day one. They try it in MCP, like it, then move it to CI. The funnel is MCP → CI, never CI → MCP.

### No Enterprise Features

No SSO. No RBAC. No multi-tenant dashboard. No SLA. Not until someone with a budget asks.

---

## Phase 1 Launch Sequence

```
Day -7    Final test: all 5 prompts work on hosted endpoint with Claude Code
Day -3    Load test hosted endpoint (500 concurrent, p99 < 500ms)
Day -1    README restructured. Hero screenshot in place. Blog post drafted.

Day 0     Publish:
          1. GitHub Release v0.1.0
          2. Homebrew formula
          3. Docker images
          4. Blog post
          5. Tweet with hero screenshot
          6. Show HN post (Sunday morning EST)

Day 1-3   Monitor:
          - Hosted endpoint latency and errors
          - GitHub stars velocity
          - Issue quality (bug vs feature request)
          - Any false positive reports (highest priority to fix)

Day 7     Retrospective:
          - How many installs?
          - How many came back after day 1?
          - What are the top 3 complaints?
          - Adjust P1 priorities based on real data
```

---

## Success Metrics (90 Days)

| Metric | Target | Why |
|--------|--------|-----|
| Hosted endpoint daily active users | 100+ | Core adoption |
| GitHub stars | 500+ | Social proof |
| "Still in settings.json after 2 weeks" rate | 30%+ | Retention = real value |
| False positive rate reported | < 5% of issues | Quality signal |
| Feature requests (non-bug issues) | 20+ | Engaged community |
| Unsolicited blog posts / tweets | 5+ | Organic growth |

**The single most important metric:** How many people still have Evidra in their Claude Code config after two weeks?

If that number is high, everything else follows. If it's low, no amount of features or marketing fixes it — the core experience needs work.

---

## Roadmap Horizon (Gated by Traction)

Nothing below is planned. It's a list of possibilities gated by real-world demand signals.

```
v0.2 (Phase 2, only if Phase 1 retention > 30%)
  - CI integration example (GitHub Actions)
  - evidra evidence last / violations
  - Improved deny messages based on real user confusion
  - Bug fixes from Phase 1 feedback

v0.3 (Phase 2-3, only if DevOps pull appears)
  - Custom bundle overlay (EVIDRA_CUSTOM_BUNDLE_PATH)
  - Evidence forwarder (webhook / SIEM)
  - Per-environment profiles documented better
  - Boss one-pager PDF

v0.4+ (Phase 3, only if enterprise asks with budget)
  - Centralized policy management
  - Team/org management
  - SSO / RBAC
  - SaaS tiers
  - Evidence anchoring
```

Each version gate is a traction signal, not a date.

---

## The Frame

You're not building a security product.

You're building the **social permission layer for AI infrastructure automation.**

The DevOps engineer doesn't want a policy engine. They want the ability to say to their boss: **"We're safe."**

The boss doesn't want AI agents. They want the ability to say to their board: **"We have controls."**

Evidra is the sentence that makes both conversations possible:

> "Every AI infrastructure action is validated against policy before execution. Every decision is logged. Dangerous operations are blocked."

MCP is how the engineer discovers it. The evidence log is how they justify it. The deny message is how they trust it.

Everything else waits.
