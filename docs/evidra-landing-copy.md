# Evidra — Positioning & Landing Page Copy

---

## Core Positioning

**One line:** Evidra is a kill-switch for AI agents managing your infrastructure.

**Problem:** AI agents (Claude Code, Cursor, Copilot) are getting access to
kubectl, terraform, helm. They're fast, helpful — and one hallucinated command
away from deleting your production database.

**Solution:** Evidra sits between the agent and your infrastructure.
Every destructive operation is validated against policy before execution.
If context is missing or the action is dangerous — it's blocked. Silently.
Before anything happens.

**Who it's for:** DevOps engineers, platform teams, and anyone who gives
an AI agent access to infrastructure tools.

**What it is NOT:** not an admission controller, not a CI gate, not a
config validator. It's a seatbelt. You don't think about it until it
saves you.

---

## Value pillars (in order of importance)

### 1. Kill-switch
Agent can't destroy what it can't get past Evidra.
Empty payload? Denied. Unknown tool? Denied. Missing namespace? Denied.
Fail-closed by default.

### 2. Golden rules
23 curated policies catch the configs that cause outages: privileged
containers, public S3 buckets, wildcard IAM, open security groups.
Not opinions — lessons from real incidents.

### 3. Evidence
Every decision — allow and deny — is recorded to a tamper-evident chain.
You always know who did what, when, and whether it was approved.

---

## Landing page structure

### Hero
**Headline:** Your AI agent is one hallucination away from `rm -rf production`
**Subhead:** Evidra is a kill-switch for AI agents managing infrastructure.
Every destructive command is validated before execution. If it's dangerous — it's blocked.
Every decision is recorded to a tamper-evident evidence chain. You always know what happened and why.
**CTA:** `pip install evidra` / View on GitHub

### The problem (3 scenarios)
- Claude Code runs `kubectl delete` in the wrong namespace → production down
- Cursor applies a terraform plan with 47 deletions it didn't understand → data gone
- Copilot opens security group 0.0.0.0/0 on port 22 → you find out from the news

### How it works (3 steps)
1. **Install** — one command, MCP server, zero config
2. **Agent calls validate** — before any destructive operation
3. **Evidra decides** — allow, deny, or "give me more context"

### What it catches
- Empty payloads (agent forgot to fill context)
- Protected namespaces (kube-system, prod)
- Mass deletions (>5 resources at once)
- Privileged containers
- Public S3 buckets without encryption
- Wildcard IAM policies (Action:*, Resource:*)
- Open security groups (0.0.0.0/0 on SSH/RDP)
- Unknown tools (new tool? denied until registered)

### Two protection levels
**baseline** — kill-switch only. Blocks empty/dangerous operations.
**ops** (default) — kill-switch + 23 golden rules. Full protection.

### Evidence
Every decision is logged. Tamper-evident. Cryptographically chained.
"Who ran this?" "Was it approved?" "What policy was active?"
You'll never need it until the postmortem. Then you'll be glad it's there.

### Footer
Open source. MIT license. No SaaS. No telemetry. Runs locally.
Your infrastructure, your rules, your evidence.

---

## Key phrases / copy snippets

**For README:**
> Evidra is a policy-based kill-switch for AI agents managing infrastructure.
> It validates every destructive operation before execution and blocks
> anything that looks dangerous, incomplete, or unauthorized.

**For GitHub description:**
> Kill-switch for AI agents. Validates infrastructure operations before
> execution. Fail-closed. Evidence-backed.

**For social/launch:**
> Your AI agent has access to kubectl and terraform.
> What happens when it hallucinates?
> Evidra: a kill-switch that blocks destructive operations before they execute.
> Open source. One command install. Fail-closed by default.

**For the "why" section:**
> AI agents are getting better at infrastructure. They're also getting
> better at making mistakes faster. Evidra doesn't slow them down —
> it stops the catastrophic ones.

**Tone rules:**
- No marketing fluff. No "revolutionary". No "next-gen".
- Direct. Technical. Slightly dark humor about things going wrong.
- Speak to the engineer who's been paged at 3am, not the VP reading a deck.
- Short sentences. No bullet points in hero copy.
