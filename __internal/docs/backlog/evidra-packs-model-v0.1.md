# Evidra --- Packs Model v0.1 (Draft)

This document defines how Evidra policies are grouped into Packs
(Bundles) for clearer onboarding, gradual adoption, and differentiated
support.

Principles: - Packs are **policy groupings**, not a different engine. -
Evidence levels are **orthogonal** to Packs. - Default UX should enable
safety with minimal friction. - Packs should map to real audiences and
entry points.

------------------------------------------------------------------------

## 1. Definitions

### Pack

A named set of policies (OPA bundle/profile) targeting a specific
audience and risk surface.

### Profile

A runtime selection that enables one or more Packs and sets policy
parameters. Example: `profile_name: devops-default`.

------------------------------------------------------------------------

## 2. Pack List (v0.1)

### Pack 1 --- Daily Killers

**Audience:** non-ops developers, AI-assisted coding workflows, local
terminals.

**Goal:** prevent common destructive commands and irreversible actions
before they happen.

**Input type:** raw command strings (shell/git/docker), minimal context
(cwd/repo/branch optional).

**Recommended tool:** `check_command` (string → pattern matching →
allow/deny).

**Examples (non-exhaustive):** - `rm -rf /`, `rm -rf ~`, `rm -rf ..` -
`git push -f` (especially to protected branches) - `git clean -fdx` -
`docker system prune -a` - `curl ... | sh` (warn/deny depending on
profile)

**Default behavior:** allow unless matched by an explicit "killer"
pattern.

**Notes:** - Keep this pack intentionally small (10--20 rules). - Avoid
full parsing; prefer robust pattern matching + hints.

------------------------------------------------------------------------

### Pack 2 --- DevOps Baseline

**Audience:** DevOps / Platform teams running infra changes via AI
agents, CI, GitOps tools.

**Goal:** kill-switch behavior for infra tools: fail-closed,
unknown-tool guardrails.

**Input type:** structured ToolInvocation (tool, operation, target,
payload).

**Recommended tool:** `validate` (OPA-based decision).

**Core rules:** - `ops.unknown_destructive` (deny) -
`ops.insufficient_context` (deny) - truncation/partial-context guards
(deny) - protected namespaces / mass delete guardrails (deny)

**Default behavior:** safe "guardrails on" with minimal required
context.

------------------------------------------------------------------------

### Pack 3 --- Ops Disaster Prevention (Golden Rules)

**Audience:** production ops/security. Teams with defined standards for
"never allow" configs.

**Goal:** enforce a curated set of high-signal safety policies (your
"Golden 20/23").

**Input type:** same as Pack 2 (structured ToolInvocation).

**Recommended tool:** `validate`.

**Examples:** - privileged containers - host namespace / hostPath -
wildcard IAM - public S3 / missing encryption - open security groups /
0.0.0.0/0 critical ports - ArgoCD dangerous sync settings

**Default behavior:** strict deny on known bad configurations, with
clear hints.

------------------------------------------------------------------------

## 3. How Packs Are Delivered

### Option A (recommended): Separate OPA bundles per Pack

-   `bundle-daily-killers`
-   `bundle-devops-baseline`
-   `bundle-ops-golden`

Pros: - clean separation - simpler release notes - users can pin
versions per pack

Cons: - more bundles to ship/test

### Option B: Single bundle + pack flags in data

Single bundle with all rules; packs enabled via policy params:

``` json
{
  "packs": {
    "enabled": ["devops", "ops"]
  }
}
```

Pros: - fewer artifacts - easier for single-file installs

Cons: - more conditional logic in rules

**v0.1 recommendation:** start with Option B (single bundle + pack
flags) until demand requires multi-bundle packaging.

------------------------------------------------------------------------

## 4. Pack Selection (Profiles)

Profiles define enabled Packs and core parameters.

Examples:

### Profile: `daily-default`

-   enabled packs: `["daily"]`
-   tool exposed: `check_command`
-   strictness: medium (warn for risky patterns, deny for destructive)

### Profile: `devops-default`

-   enabled packs: `["devops"]`
-   tool exposed: `validate`
-   strictness: fail-closed for destructive operations

### Profile: `ops-strict`

-   enabled packs: `["devops", "ops"]`
-   tool exposed: `validate`
-   strictness: strict, minimal breakglass

------------------------------------------------------------------------

## 5. Evidence and Packs

Evidence level does not depend on pack. However, evidence records should
include:

-   `profile_name`
-   `bundle_revision`
-   `policy_ref`

This allows answering: - "Which pack/profile was active when this
decision happened?"

------------------------------------------------------------------------

## 6. Adoption Path

Recommended onboarding flow:

1.  Start with Pack 2 (DevOps Baseline) for AI infra safety.
2.  Add Pack 3 (Ops Golden Rules) once teams agree on "never allow"
    rules.
3.  Offer Pack 1 (Daily Killers) as optional developer safety add-on.

This avoids introducing raw-command tooling on day one.

------------------------------------------------------------------------

## 7. Non-Goals (v0.1)

-   Full shell parser
-   Exec/orchestration (Evidra remains validate-only)
-   SaaS components
-   Marketplace / remote pack registry (later)

------------------------------------------------------------------------

END OF DOCUMENT v0.1
