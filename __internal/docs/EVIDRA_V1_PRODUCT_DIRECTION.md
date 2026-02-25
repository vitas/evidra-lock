# Evidra v1 --- Product Direction (DevOps-Focused)

Generated on: 2026-02-22T19:41:40.591465 UTC

------------------------------------------------------------------------

## 🎯 Product Focus

**Evidra v1 is a deterministic outcome validator for AI-generated
infrastructure changes.**

Target user: **DevOps engineer using AI tools** (Copilot, ChatGPT,
internal LLM agents).

Not: - A compliance platform - A SOC2 automation suite - A generic AI
gateway - A DevSecOps mega-framework

------------------------------------------------------------------------

## 🧠 Core Problem

AI agents can propose infrastructure changes.\
AI is probabilistic. Infrastructure is not.

Without a deterministic safety layer: - AI may hallucinate destructive
commands - AI may open public access - AI may delete production
resources - Logs are not proof of control

------------------------------------------------------------------------

## ✅ Core Value Proposition

Evidra validates the *outcome* of infrastructure changes before they
run.

It does not filter command strings.

It evaluates structured plan/diff descriptions of the intended changes. (for example: Terraform plan JSON - Kubernetes diff)

Then applies deterministic policy and returns:

-   PASS / FAIL
-   Risk level
-   Human-readable reason
-   Immutable evidence record

------------------------------------------------------------------------

## 📦 MVP Scope (Must Have Before Release)

### 1️⃣ Outcome-Based Validation

-   Evaluate structured plan/diff artifacts (for example Evaluate Terraform plan JSON and Kubernetes diff)
-   Decision based on resulting changes, not commands

### 2️⃣ Deterministic Decision Output

Each validation returns: - PASS / FAIL - risk_level - reason
(human-readable) - structured AI-readable error

### 3️⃣ Immutable Evidence

Every decision produces: - Append-only record - Hash-linked chain -
Machine-readable JSON

### 4️⃣ AI-Readable Structured Error

Instead of generic errors, return structured output like:

``` json
{
  "blocked_by": "ops.unapproved_change",
  "reason": "Production changes require approval",
  "required_changes": [
    "Add tag change-approved=true"
  ]
}
```

------------------------------------------------------------------------

## ❌ Out of Scope for v1

The following features are intentionally excluded from v1:

-   SOC2/HIPAA marketing narratives
-   CI passive training mode
-   JSON-LD export
-   Jira integrations
-   Splunk integrations
-   Signed export artifacts
-   Approval workflow systems
-   Compliance dashboards
-   Enterprise governance layers

------------------------------------------------------------------------

## 🧱 Simplified Architecture (v1)

Keep: - Engine (pipeline) - Policy (small deny/warn rules) - Registry
(clean, declarative) - Validators (facts only) - Evidence store

Remove or de-emphasize: - Dev/demo tools - Multi-profile complexity -
Compliance-first messaging

------------------------------------------------------------------------

## 🧪 Example UX

``` bash
terraform plan -out plan.tfplan
terraform show -json plan.tfplan > plan.json

evidra validate plan.json
```

Output:

    FAIL
    Reason: This change deletes 3 RDS instances.
    Risk: high
    Evidence: ev-2026-...

------------------------------------------------------------------------

## 🧭 Strategic Principle

If a feature does not strengthen:

> Deterministic validation of infrastructure outcomes

It does not belong in v1.

------------------------------------------------------------------------

## 📊 Honest Assessment

Technical maturity: High\
Product clarity: Needs sharpening\
MVP focus: Must tighten\
Risk of feature creep: High

------------------------------------------------------------------------

## 🚀 Release Readiness Checklist

Before release:

-   [ ] Policy simplified into small deny/warn rules
-   [ ] Serious Baseline Policy Pack: 23 rules across K8s, Terraform, S3, IAM, ArgoCD
        (see `docs/backlog/OPS_V0_SERIOUS_BASELINE_RESEARCH.md`)
-   [ ] Registry cleaned and fully tested
-   [ ] Observe vs Enforce semantics tested
-   [ ] README simplified to 1 clear workflow
-   [ ] Remove non-core features from messaging

------------------------------------------------------------------------

## 🏁 Final Positioning Statement

> Validate infrastructure changes proposed by AI before they run.

Short. Clear. Deterministic.
