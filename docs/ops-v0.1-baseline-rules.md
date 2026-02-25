# Ops-v0.1 Baseline Runtime Profile

## Purpose

The `ops-v0.1` profile provides 18 catastrophic guardrails plus 5 operational rules
for AI-generated and automated infrastructure changes.

This profile is intentionally small.

It focuses only on catastrophic, high-signal misconfigurations that:

- Cause production outages or irreversible data loss
- Expose infrastructure or data publicly
- Break GitOps safety guarantees
- Enable cluster or account compromise

It is designed to be:

- Deterministic (no runtime API calls, no probabilistic analysis)
- Low false-positive (16 of 18 guardrail rules have Low FP risk)
- Easy to explain (18 rules a security team can audit in 30 minutes)
- Safe for early adoption

---

## What Evidra Is Not

Evidra is **not** a CIS compliance scanner. It does not replace tfsec, trivy, kube-bench, or Checkov.

These tools audit hundreds of controls across entire infrastructure. Evidra evaluates
AI agent behavior at the point of action — a single tool invocation at a time.

The 18 guardrail rules were selected because:

1. Every rule has a specific, named incident or documented attack chain.
2. Every rule is deterministically evaluable from static configuration.
3. The rule count is small enough to explain, audit, and defend in one meeting.
4. Every rule maps to a CIS control, tfsec AVD ID, or kube-score check.

---

## Rule Summary

### Deny Rules (18)

| Rule ID | Domain | Scope | Description |
|---|---|---|---|
| `ops.mass_delete` | Terraform/K8s | all | Bulk-delete exceeds threshold |
| `terraform.sg_open_world` | Terraform | all | SG 0.0.0.0/0 on SSH/RDP |
| `terraform.s3_public_access` | Terraform | all | S3 missing Block Public Access |
| `terraform.iam_wildcard_policy` | Terraform | all | IAM wildcard Action or Resource |
| `k8s.privileged_container` | Kubernetes | all | Container privileged: true |
| `k8s.host_namespace_escape` | Kubernetes | all | hostPID / hostIPC / hostNetwork |
| `k8s.run_as_root` | Kubernetes | all | Container runs as UID 0 |
| `k8s.hostpath_mount` | Kubernetes | all | hostPath volume mount |
| `k8s.dangerous_capabilities` | Kubernetes | all | SYS_ADMIN / SYS_PTRACE / NET_RAW |
| `argocd.autosync_prod` | ArgoCD | prod | Automated sync in production |
| `argocd.wildcard_destination` | ArgoCD | all | Wildcard namespace/server |
| `argocd.dangerous_sync_combo` | ArgoCD | prod | auto-sync + prune + selfHeal |
| `s3.no_encryption` | S3 | all | Missing server-side encryption |
| `s3.no_versioning_prod` | S3 | prod | Versioning disabled in production |
| `iam.wildcard_policy` | IAM | all | Action:\* + Resource:\* (root) |
| `iam.wildcard_principal` | IAM | all | Trust policy Principal:\* |
| `k8s.protected_namespace` | Kubernetes | all | Changes in restricted namespace |
| `ops.unapproved_change` | Ops | all | Protected ns without approval |

### Warn Rules (2 guardrail + 3 operational)

| Rule ID | Domain | Description |
|---|---|---|
| `k8s.mutable_image_tag` | Kubernetes | :latest or untagged image |
| `k8s.no_resource_limits` | Kubernetes | Missing CPU/memory limits |
| `ops.autonomous_execution` | Ops | Agent via MCP (audit) |
| `ops.breakglass_used` | Ops | Breakglass tag present (audit) |
| `ops.public_exposure` | Ops | Terraform public exposure |

---

## Coverage Map

| Attack Surface | Rules |
|---|---|
| Container escape → host | `k8s.privileged_container`, `k8s.host_namespace_escape`, `k8s.run_as_root`, `k8s.hostpath_mount`, `k8s.dangerous_capabilities` |
| Mass data exposure | `terraform.s3_public_access`, `s3.no_encryption`, `iam.wildcard_policy`, `iam.wildcard_principal` |
| Irreversible destruction | `ops.mass_delete`, `argocd.dangerous_sync_combo`, `s3.no_versioning_prod` |
| Account/cluster takeover | `terraform.iam_wildcard_policy`, `k8s.privileged_container`, `iam.wildcard_policy`, `iam.wildcard_principal` |
| Network exposure | `terraform.sg_open_world`, `ops.public_exposure` |
| GitOps operational safety | `argocd.autosync_prod`, `argocd.wildcard_destination`, `argocd.dangerous_sync_combo` |
| Supply chain / integrity | `k8s.mutable_image_tag` |
| Availability / DoS | `k8s.no_resource_limits` |

---

## Profile Design Principles

1. High signal only — no noise.
2. No compliance checklist sprawl.
3. Focus on catastrophic impact.
4. Deterministic evaluation.
5. Environment-aware where applicable.
6. Every rule has a documented real-world incident or attack chain.

---

## Non-Goals (v0.1)

This profile does NOT aim to:

- Fully implement CIS benchmarks
- Replace dedicated security scanners
- Cover every Kubernetes best practice
- Enforce organization-specific policies
- Perform runtime or network-based detection
