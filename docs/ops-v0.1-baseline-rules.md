# Ops-v0.1 Baseline Runtime Profile

## Purpose

The `ops-v0.1` profile provides a minimal, high-impact set of guardrails
for AI-generated and automated infrastructure changes.

This profile is intentionally small.

It focuses only on catastrophic, high-signal misconfigurations that:

- Cause production outages
- Expose infrastructure publicly
- Break GitOps safety guarantees
- Introduce severe security risks

It is designed to be:

- Deterministic
- Low false-positive
- Easy to explain
- Safe for early adoption

---

# Terraform Rules

## 1. terraform.mass_delete

**Severity:** High  
**Action:** Deny  

**Condition:**
- Number of delete actions exceeds threshold (e.g., > 1 in production)

**Why it exists:**
Mass deletion is one of the most common catastrophic mistakes in Terraform plans.

---

## 2. terraform.sg_open_world

**Severity:** High  
**Action:** Deny  

**Condition:**
- Security group ingress from `0.0.0.0/0`
- Port 22 (SSH) or 3389 (RDP)
- Environment is production

**Why it exists:**
Public SSH/RDP exposure is a major security risk.

---

## 3. terraform.s3_public_access

**Severity:** High  
**Action:** Deny  

**Condition:**
- S3 bucket ACL is public
- OR Public Access Block disabled
- OR bucket policy allows `"Principal": "*"`

**Why it exists:**
Public S3 buckets are a common source of data leaks.

---

# Kubernetes / kubectl Rules

## 4. k8s.privileged_container

**Severity:** High  
**Action:** Deny  

**Condition:**
- `securityContext.privileged = true`

**Why it exists:**
Privileged containers can escape container isolation.

---

## 5. k8s.run_as_root

**Severity:** High  
**Action:** Deny  

**Condition:**
- `runAsUser = 0`
- OR `runAsNonRoot` is not explicitly true

**Why it exists:**
Running as root increases blast radius of compromise.

---

## 6. k8s.mutable_image_tag

**Severity:** Medium  
**Action:** Deny (or Warn, depending on policy strictness)

**Condition:**
- Image tag is `latest`
- OR no explicit tag specified

**Why it exists:**
Mutable tags break reproducibility and GitOps guarantees.

---

# ArgoCD Rules

## 7. argocd.autosync_prod

**Severity:** High  
**Action:** Deny  

**Condition:**
- `spec.syncPolicy.automated` enabled
- AND destination environment is production

**Why it exists:**
Automatic sync in production removes human safety checks.

---

## 8. argocd.wildcard_destination

**Severity:** High  
**Action:** Deny  

**Condition:**
- Destination namespace = `*`
- OR destination cluster = `*`

**Why it exists:**
Wildcard destinations allow uncontrolled deployment scope.

---

# S3 Standalone Rules (if evaluated outside Terraform)

## 9. s3.no_encryption

**Severity:** High  
**Action:** Deny  

**Condition:**
- Server-side encryption not configured

**Why it exists:**
Unencrypted storage increases data exposure risk.

---

# Profile Design Principles

The `ops-v0.1` profile follows these principles:

1. High signal only — no noise.
2. No compliance checklist sprawl.
3. Focus on catastrophic impact.
4. Deterministic evaluation.
5. Environment-aware where applicable.

---

# Non-Goals (v0.1)

This profile does NOT aim to:

- Fully implement CIS benchmarks
- Replace dedicated security scanners
- Cover every Kubernetes best practice
- Enforce organization-specific policies

---

# Future Expansion (Optional)

Future versions may include:

- Helm-rendered manifest evaluation
- Kustomize transformation validation
- Terraform drift detection safeguards
- Policy pack modularization
- Environment-based severity overrides

---

# Summary

The `ops-v0.1` profile provides a practical and production-oriented
baseline guardrail set for AI-driven infrastructure changes.

It is intentionally small.

It is intentionally strict.

It is intentionally deterministic.