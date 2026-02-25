# Ops Guardrails Strategy and External References

## Purpose

This document explains:

1. Where the `ops-v0.1` guardrails are inspired from.
2. Why Evidra does NOT implement full compliance benchmarks.
3. How to select high-impact rules without turning the project into a compliance scanner.
4. What the minimal, production-grade baseline should include.

Evidra is not a compliance engine.

It is a deterministic blast-radius limiter for AI-generated infrastructure changes.

---

# Design Philosophy

Evidra guardrails follow these principles:

- High signal only.
- Catastrophic impact first.
- Low false positive.
- Deterministic evaluation.
- Simple mental model.
- Lightweight runtime.

We explicitly avoid:

- Full CIS benchmark implementation.
- Hundreds of low-impact checks.
- Compliance checklist sprawl.
- Replacing dedicated security scanners.

---

# Authoritative Industry Sources

The following sources inform rule selection.

These are references — not full rule imports.

---

## Kubernetes

### CIS Kubernetes Benchmark

Published by the Center for Internet Security.

Focus areas relevant to Evidra:

- Privileged containers
- Host namespace usage
- Root execution
- Dangerous Linux capabilities
- Admission misconfigurations

We only extract high-blast-radius misconfigurations.

---

### kube-bench (Aqua Security)

Automated CIS validation implementation.

Useful for identifying which controls matter most in real-world clusters.

---

### kube-score (Zalando)

Practical workload scoring tool.

High-impact checks to consider:

- Privileged containers
- Missing security context
- HostPath volumes
- Missing resource limits
- Missing probes

Only a small subset should be used.

---

## Terraform

### tfsec (Aqua Security)

Terraform static analysis tool.

Relevant high-impact patterns:

- Open security groups
- Public S3 buckets
- Missing encryption
- Weak IAM policies

Avoid importing large rule sets. Select only catastrophic patterns.

---

## S3 / AWS

AWS official security best practices recommend:

- Block Public Access
- Server-side encryption
- Versioning
- Restrictive bucket policies

Only encryption and public exposure are considered mandatory in baseline.

---

## ArgoCD

Argo Project documentation highlights operational risks around:

- Automated sync in production
- SelfHeal + Prune combinations
- Wildcard project destinations
- Broad RBAC scopes

These are strong candidates for deterministic deny rules.

---

# Catastrophic Guardrails Pack (Recommended Baseline)

This is the minimal, production-grade rule set.

It prioritizes real-world outages and severe security incidents.

---

## Terraform

- terraform.mass_delete  
  Deny excessive delete actions in production.

- terraform.sg_open_world  
  Deny SSH/RDP exposed to 0.0.0.0/0 in production.

- terraform.s3_public_access  
  Deny public S3 buckets or disabled Public Access Block.

- terraform.no_encryption  
  Deny storage resources without encryption.

---

## Kubernetes (kubectl / Helm / Kustomize after render)

- k8s.privileged_container  
  Deny privileged containers.

- k8s.run_as_root  
  Deny containers running as root.

- k8s.host_namespace_escape  
  Deny hostNetwork or hostPID usage in production.

- k8s.mutable_image_tag  
  Deny or warn on image tag `latest` or missing tag.

Helm and Kustomize should be evaluated post-render using the same Kubernetes rules.

---

## ArgoCD

- argocd.autosync_prod  
  Deny automated sync in production environments.

- argocd.wildcard_destination  
  Deny wildcard cluster or namespace targets.

- argocd.dangerous_sync_combo  
  Deny automated + prune + selfHeal combination in production.

---

## S3 (Standalone Evaluation)

- s3.no_encryption  
  Deny buckets without server-side encryption.

- s3.no_versioning_prod  
  Deny versioning disabled in production.

---

## Full Research (18 Rules)

The structured research across all sources above is in:
[`OPS_V0_SERIOUS_BASELINE_RESEARCH.md`](./OPS_V0_SERIOUS_BASELINE_RESEARCH.md)

That document expands the baseline below to 18 rules (17 new + 1 existing),
adding: `k8s.privileged_container`, `k8s.host_namespace_escape`, `k8s.run_as_root`,
`k8s.hostpath_mount`, `k8s.dangerous_capabilities`, `k8s.mutable_image_tag`,
`k8s.no_resource_limits`, `terraform.sg_open_world`, `terraform.s3_public_access`,
`terraform.iam_wildcard_policy`, `s3.no_encryption`, `s3.no_versioning_prod`,
`iam.wildcard_policy`, `iam.wildcard_principal`, `argocd.autosync_prod`,
`argocd.wildcard_destination`, `argocd.dangerous_sync_combo`.

Each rule includes: source inspiration (CIS/AVD/kube-score IDs), deterministic evaluation
strategy, expected FP risk, and production-scope condition.

---

# Why This Set Is Enough

This baseline:

- Covers the most common catastrophic misconfigurations.
- Prevents irreversible production mistakes.
- Stops public data exposure.
- Preserves GitOps safety.
- Keeps rule count small and maintainable.

More rules do not necessarily increase safety.
They increase noise and reduce trust.

---

# Explicit Non-Goals

This guardrail set does NOT aim to:

- Fully implement CIS benchmarks.
- Replace tfsec, kube-bench, or cloud security scanners.
- Provide compliance certification.
- Cover every Kubernetes best practice.
- Enforce organization-specific custom logic.

---

# When to Add More Rules

Add new rules only if:

1. They prevent catastrophic production failure.
2. They prevent severe security exposure.
3. They have low false positive rates.
4. They are deterministic.
5. They align with the local-first philosophy.

Avoid feature creep.

---

# Strategic Positioning

Evidra is not:

- A compliance scanner.
- A full security posture management platform.
- A policy-as-code SaaS platform (yet).

Evidra is:

A deterministic guardrail for AI-generated infrastructure changes.

That clarity keeps the product lightweight, adoptable, and sharp.