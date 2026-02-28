# Evidra README — Rewrite Instructions

The current README positions Evidra as "a guardrail layer." The new
positioning is **kill-switch for AI agents managing infrastructure.**

These instructions describe what to change and why. The README structure
stays similar — the content shifts to match landing page positioning.

Reference: `evidra-landing.html` and `evidra-landing-copy.md` for tone
and copy.

---

## File to edit

`README.md` (project root)

---

## Tone rules

- Direct. Technical. Short sentences.
- Speak to the engineer, not the VP.
- No marketing fluff. No "revolutionary", "next-gen", "cutting-edge".
- Slightly dark humor about things going wrong is ok.
- Simple English — not everyone is a native speaker.
  Avoid jargon like "hallucination", "postmortem", "attestation".
- Keep code examples real and runnable.

---

## Section-by-section changes

### 1. Title + one-liner

**Current:**
```
# Evidra
Evidra is a guardrail layer between AI agents and your infrastructure.
```

**New:**
```
# Evidra

Kill-switch for AI agents managing infrastructure.

Your infrastructure. Your rules. Your evidence.
```

### 2. What it does (replace the bullet list)

**Current:** 4 generic bullets about OPA rules, deny responses, evidence, MCP.

**New — 3 pillars, punchier:**
```
AI can suggest. Evidra decides.

- **Kill-switch** — destructive operations are blocked unless proven safe.
  Empty payload? Denied. Unknown tool? Denied. Ambiguous target scope? Denied.
- **23 golden rules** — curated policies that catch the configs behind
  real outages: privileged containers, public S3, wildcard IAM, open
  security groups.
- **Evidence** — every decision (allow and deny) is appended to a
  SHA-256 hash-chained log. Each record references the hash of the
  previous record. Tampering breaks verification.
```

### 3. Why It Exists

**Current:** good content, but says "validation layer in the loop."

**New — same message, engineering tone:**
```
## Why

AI agents now run kubectl, terraform, and argocd on production
infrastructure. They may produce incomplete, unsafe, or semantically
incorrect actions.
Humans may approve without understanding full impact. CI pipelines
may apply plans automatically.

Evidra sits between the agent and your infrastructure. Every
destructive command is validated against explicit policy before
execution. If it's dangerous, incomplete, or unknown — it's denied.

Evidra does not rely on natural language analysis. It evaluates
structured tool invocations against OPA policy. If the input cannot
be mapped to policy with sufficient context, the request is denied.

Input must be structured. Evidra does not parse raw shell commands.
Tool + operation + payload are explicit.
```

### 4. Fail-closed by default (NEW SECTION — add after Why)

```
## Fail-closed by default

If Evidra cannot evaluate the operation safely, it denies.

- Unknown tool or operation → deny
- Missing payload fields on destructive operation → deny
- Truncated or incomplete input → deny
- Ambiguous target scope (namespace not provided and cannot be inferred) → deny

Scope can be provided explicitly (`target.namespace`) or via resolved
context (`context.namespace` from current kubectl/oc context).

Silence is not allow. Explicit allow is required for execution.
Read-only operations (get, list, plan, describe) are allowed by default.
```

### 5. 30-Second Demo

Keep as-is — it's already good. Just update the intro line:

**Current:** "Install Evidra and connect it to Claude Code (see below)."

**New:** "Install Evidra. Connect to Claude Code. Try to break things."

### 5. Install

Keep as-is — already has brew, docker, go install.

### 6. Connect to Claude Code

Keep as-is.

### 7. How It Works

**Current:** generic flow diagram.

**New — add MCP positioning + engineering details:**
```
## How it works

Evidra runs as a standard MCP server. AI agents discover it
automatically and call `validate` before destructive operations.

```
AI agent → MCP: validate → Evidra (OPA policy) → allow/deny → evidence chain
                                                ↓
                                    only if allowed → kubectl / terraform / helm
```

1. Agent sends tool invocation via MCP before executing.
2. Evidra maps tool + operation to intent (destructive or read-only).
3. Request is evaluated against a versioned OPA policy bundle.
4. Decision returned: allow/deny + risk level + rule IDs + hints.
5. Decision recorded to append-only, hash-linked evidence log.

Evaluation is deterministic and fail-closed. No runtime API calls.
No network dependencies. No LLM in the evaluation loop.
The policy bundle is versioned and ships embedded in the binary.

The agent sees rule IDs and actionable hints on every deny —
not just "no."

**Also works in CI pipelines:**

```bash
terraform show -json tfplan | evidra validate --tool terraform --op apply
```

Same policy engine. Same evidence chain. Works in AI workflows and
traditional CI pipelines. In CI mode, Evidra operates without MCP —
the same validation engine is called directly.
```

### 8. Protection Levels (NEW SECTION — add after How It Works)

```
## Protection levels

Evidra ships with two levels. Default is maximum safety.

**ops** (default) — full protection. Kill-switch guardrails plus 23
curated rules that catch privileged containers, public S3 buckets,
wildcard IAM, open security groups, and other catastrophic
misconfigurations.

**baseline** — kill-switch only. Blocks destructive operations with
missing context and unknown tools. No opinion on what's "bad config."

Switch with one env var:

```bash
EVIDRA_PROFILE=baseline  # kill-switch only
EVIDRA_PROFILE=ops       # default — full protection
```
```

### 9. Policy Baseline

**Current:** good. Rename to "Policy catalog" and add one line:

Add at the start:
```
Not a compliance scanner. Every rule prevents a specific high-impact
failure that has caused real outages.
```

Keep the rest. Keep the link to POLICY_CATALOG.md.

### 10. Not an admission controller (NEW SECTION — add after Policy)

```
## Not a replacement for OPA

Admission controllers run at deploy time, inside the cluster.
Evidra runs before execution — across kubectl, terraform, helm,
argocd. Especially in AI-driven workflows. Keep both.
```

### 11. Threat model (NEW SECTION — add after Not OPA)

```
## Threat model

Evidra assumes:
- AI agents may generate incomplete or unsafe infrastructure actions.
- Payload may be incorrect, partial, or adversarial.
- Humans may approve without understanding full impact.
- CI pipelines may apply plans automatically.

Evidra does not:
- Execute commands.
- Modify infrastructure.
- Make network calls during evaluation.
- Replace admission controllers or CI gates.

It validates structured input against explicit policy, and records
the decision. Nothing more.

Evidra does not sit in the execution path. It must be called
before execution by the agent, CLI, or CI pipeline.
```

### 12. Evidence (NEW SECTION — add after Threat model)

```
## Evidence

Every decision — allow and deny — is recorded to a local,
append-only evidence log.

- SHA-256 hash chain: each record includes the hash of the
  previous record.
- Immutable: append-only file, no updates, no deletes.
- Tamper detection: `evidra evidence verify` checks the full chain.
- Offline: stored locally, no external service required.
- Contains: actor, operation, policy decision, rule IDs, timestamps,
  payload digest (not raw payload by default).
  Raw payload storage is optional and configurable.

```bash
# Verify the evidence chain is intact
evidra evidence verify

# Export for external audit
evidra evidence export --format json
```
```

### 13. CLI

Keep as-is.

### 14. License + footer

**Current:** "Apache License 2.0"

**New:**
```
## License

Apache 2.0. No SaaS required. No telemetry. Runs locally or on-prem.

Your infrastructure, your rules, your evidence.

---
Open source by [SameBits](https://samebits.com).
```

---

## GitHub repo description (Settings → About)

**Current:** (unknown)

**New:**
```
Kill-switch for AI agents. Validates infrastructure operations before execution. Fail-closed. Evidence-backed.
```

---

## GitHub topics (Settings → About → Topics)

```
ai-safety, mcp, infrastructure, devops, policy, opa, kubernetes,
terraform, kill-switch, guardrails
```

---

## Checklist

After editing:

- [ ] Title says "kill-switch", not "guardrail layer"
- [ ] "Your infrastructure. Your rules. Your evidence." is visible
- [ ] Three pillars: kill-switch, golden rules, evidence
- [ ] Evidence bullet mentions SHA-256 hash chain specifically
- [ ] "Fail-closed by default" section exists with 4 deny cases
- [ ] "Does not rely on natural language analysis" statement present
- [ ] "Deterministic and fail-closed. No LLM in the evaluation loop."
- [ ] MCP positioning: "standard MCP server, agents discover automatically"
- [ ] CI example: `terraform show -json | evidra validate`
- [ ] Protection levels section exists (ops/baseline)
- [ ] "Not a replacement for OPA" section exists
- [ ] Threat model section exists (assumes/does not)
- [ ] Evidence section has technical details (SHA-256, append-only, verify CLI)
- [ ] Apache 2.0 + "runs locally or on-prem"
- [ ] No unnecessary jargon: no "hallucination", no "postmortem"
- [ ] Engineering tone throughout — no "one wrong guess away"
- [ ] Demo section still works and is runnable
- [ ] All existing links (POLICY_CATALOG.md, GitHub releases) still valid
