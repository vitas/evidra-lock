> Part of the Evidra-Lock OSS toolset by SameBits.

# Policy Catalog — ops-v0.1

Bundle revision: `ops-v0.1.0-dev`
Bundle root: `policy/bundles/ops-v0.1/`
Last updated: 2026-02-25

---

## 1. Overview

### What this is (and what it is not)

Evidra-Lock is **not** a CIS compliance scanner. It does not aim to cover every best-practice checkbox. These are **catastrophic guardrails** — a small, high-signal set of rules that prevent the most devastating infrastructure misconfigurations: production outages, mass data exposure, irreversible destruction, and cluster/account compromise.

Every rule in this bundle satisfies all of the following:

| Criterion | Threshold |
|---|---|
| **Prevents catastrophic outcome** | Production outage, mass data exposure, irreversible destruction, or cluster/account compromise |
| **Deterministic evaluation** | Evaluable from static configuration (manifests, plan JSON, YAML) without runtime API calls |
| **Low false-positive rate** | Expected FP rate is Low or Medium; no rule with inherently High FP rates is included |
| **Clear blast radius** | The misconfiguration has documented, real-world attack chains or incident history |
| **Not hygiene** | Prevents a specific catastrophic scenario, not a general "best practice" |

### Why only these rules

A curated set of ops rules covers the critical attack surfaces where AI-generated infrastructure changes cause the most damage. The set stays small enough to explain in a 30-minute meeting, audit in a single code review, and defend to a security team, and remains extensible as teams add their own rules. Every rule maps to a CIS control, tfsec/trivy AVD ID, or kube-score check — nothing is invented, everything is curated.

### Bundle layout

```
policy/bundles/ops-v0.1/
├── .manifest                    — revision, roots, profile metadata
├── evidra/policy/
│   ├── policy.rego              — exports the decision object
│   ├── decision.rego            — aggregates deny/warn results
│   ├── defaults.rego            — shared helpers (resolve_param, has_tag, action_namespace, all_containers)
│   └── rules/                   — one file per rule
├── evidra/data/params/          — tunable parameter values
├── evidra/data/rule_hints/      — human-readable remediation hints
└── tests/                       — OPA test suite (70 tests)
```

### How rules, params, and hints work

- **Rule IDs** use `domain.invariant_name` format (e.g., `k8s.privileged_container`). Stable once released.
- **Params** live in `evidra/data/params/data.json` with `by_env` maps. Accessed via `resolve_param` / `resolve_list_param`.
- **Hints** live in `evidra/data/rule_hints/data.json`. 1–3 actionable strings per rule, returned in decision output.
- **Environment** (`input.environment`) selects env-specific param values. No env names are hardcoded in rules.

---

## 2. Quick Index

| # | Rule ID | Domain | Type | Prod-only | Short Description |
|---|---|---|---|---|---|
| 1 | `ops.mass_delete` | ops | deny | no | Bulk-delete exceeds threshold |
| 2 | `terraform.sg_open_world` | terraform | deny | no | Security group 0.0.0.0/0 on SSH/RDP |
| 3 | `terraform.s3_public_access` | terraform | deny | no | S3 missing Block Public Access |
| 4 | `terraform.iam_wildcard_policy` | terraform | deny | no | IAM wildcard Action or Resource |
| 5 | `k8s.privileged_container` | k8s | deny | no | Container privileged: true |
| 6 | `k8s.host_namespace_escape` | k8s | deny | no | hostPID / hostIPC / hostNetwork |
| 7 | `k8s.run_as_root` | k8s | deny | no | Container runs as UID 0 |
| 8 | `k8s.hostpath_mount` | k8s | deny | no | hostPath volume mount |
| 9 | `k8s.dangerous_capabilities` | k8s | deny | no | SYS_ADMIN / SYS_PTRACE / NET_RAW |
| 10 | `k8s.mutable_image_tag` | k8s | warn | no | :latest or untagged image |
| 11 | `k8s.no_resource_limits` | k8s | warn | no | Missing CPU/memory limits |
| 12 | `argocd.autosync_prod` | argocd | deny | yes | Automated sync in production |
| 13 | `argocd.wildcard_destination` | argocd | deny | no | Wildcard namespace/server in AppProject |
| 14 | `argocd.dangerous_sync_combo` | argocd | deny | yes | auto-sync + prune + selfHeal |
| 15 | `aws_s3.no_encryption` | aws_s3 | deny | no | S3 missing server-side encryption |
| 16 | `aws_s3.no_versioning_prod` | aws_s3 | deny | yes | S3 versioning disabled in production |
| 17 | `aws_iam.wildcard_policy` | aws_iam | deny | no | IAM Action:\* + Resource:\* (effective root) |
| 18 | `aws_iam.wildcard_principal` | aws_iam | deny | no | Trust policy Principal:\* |
| 19 | `k8s.protected_namespace` | k8s | deny | no | Changes in restricted namespace |
| 20 | `ops.unapproved_change` | ops | deny | no | Protected namespace without approval |
| 21 | `ops.public_exposure` | ops | deny | no | Terraform public exposure |
| 22 | `ops.autonomous_execution` | ops | warn | no | Agent via MCP (audit) |
| 23 | `ops.breakglass_used` | ops | warn | no | Breakglass tag present (audit) |

**Summary:** Curated deny and warn guardrails with operational audit rules. Extensible as the ops layer evolves.

---

## 3. Catastrophic Baseline Rules (18)

### Terraform / Infrastructure-as-Code

---

#### ops.mass_delete

**Intent:** Enforces an upper bound on resources deleted/destroyed in a single operation.

**Trigger:** `kubectl.delete` with `resource_count` or `terraform.plan` with `destroy_count` exceeding threshold, without `breakglass` tag.

| Param | Default | Production |
|---|---|---|
| `ops.mass_delete.max_deletes` | 5 | 3 |

**Hints:** Reduce deletion scope · Or add risk_tag: breakglass

---

#### terraform.sg_open_world

**Intent:** Deny security groups with 0.0.0.0/0 or ::/0 ingress on SSH (22), RDP (3389), or configurable dangerous ports.

**Trigger:** `terraform.plan` action with `security_group_rules` containing world-open CIDR on a port in the dangerous ports list.

| Param | Default |
|---|---|
| `terraform.sg_open_world.dangerous_ports` | [22, 3389] |

**Hints:** Restrict ingress CIDR to specific IP ranges · Use a bastion host or VPN for SSH/RDP access

**Source:** tfsec AVD-AWS-0107, CIS AWS Benchmark

---

#### terraform.s3_public_access

**Intent:** Deny S3 buckets without all four Block Public Access flags enabled.

**Trigger:** `terraform.plan` action with `resource_type: "aws_s3_bucket"` where `s3_public_access_block` is missing or incomplete, without `approved_public` tag.

**Hints:** Enable all four S3 Block Public Access settings · Add risk_tag: approved_public if bucket must be public

**Source:** tfsec AVD-AWS-0086/0087/0091/0093, CIS AWS 2.1.5

---

#### terraform.iam_wildcard_policy

**Intent:** Deny IAM policies with wildcard Action or wildcard Resource in Allow statements.

**Trigger:** `terraform.plan` action with `iam_policy_statements` containing `effect: "Allow"` with `action: "*"` or `resource: "*"`.

**Hints:** Scope IAM Action and Resource to specific services and ARNs · Use least-privilege policies

**Source:** tfsec AVD-AWS-0057, Checkov CKV_AWS_355/CKV_AWS_356

---

### Kubernetes

---

#### k8s.privileged_container

**Intent:** Deny containers with `privileged: true`. A privileged container has full host access — the container boundary ceases to exist.

**Trigger:** `k8s.apply` action where any container or init container has `security_context.privileged == true`.

**Hints:** Remove securityContext.privileged or set it to false · Use specific capabilities instead

**Source:** CIS 5.2.1, kube-bench 5.2.1

---

#### k8s.host_namespace_escape

**Intent:** Deny pods using host namespaces (hostPID, hostIPC, hostNetwork). Each individually enables container-to-host escape.

**Trigger:** `k8s.apply` action where `host_pid`, `host_ipc`, or `host_network` is `true`.

**Hints:** Remove hostPID, hostIPC, and hostNetwork from the pod spec · Use CNI plugins or service meshes

**Source:** CIS 5.2.2/5.2.3/5.2.4, kube-bench 5.2.2/5.2.3/5.2.4

---

#### k8s.run_as_root

**Intent:** Deny containers explicitly running as UID 0. Running as root makes container escape exploits practical.

**Trigger:** `k8s.apply` action where any container has `security_context.run_as_user == 0`.

**Hints:** Set securityContext.runAsUser to a non-zero UID · Add USER directive to Dockerfile

**Source:** CIS 5.2.6, kube-bench 5.2.6

---

#### k8s.hostpath_mount

**Intent:** Deny hostPath volume mounts. A container with `hostPath: /` can read/write the entire host filesystem.

**Trigger:** `k8s.apply` action where any volume has `host_path` defined.

**Hints:** Use PersistentVolumeClaims instead of hostPath · Restrict hostPath to infrastructure DaemonSets

**Source:** CIS 5.2.13, kube-bench 5.2.12

---

#### k8s.dangerous_capabilities

**Intent:** Deny containers adding SYS_ADMIN, SYS_PTRACE, or NET_RAW capabilities.

**Trigger:** `k8s.apply` action where any container has `security_context.capabilities.add` containing a capability from the dangerous list.

| Param | Default |
|---|---|
| `k8s.dangerous_capabilities.list` | ["SYS_ADMIN", "SYS_PTRACE", "NET_RAW"] |

**Hints:** Remove dangerous capabilities from capabilities.add · Use capabilities.drop: [ALL] and add back only what is needed

**Source:** CIS 5.2.7/5.2.8/5.2.9, kube-bench 5.2.7/5.2.8/5.2.9

---

#### k8s.mutable_image_tag

**Intent:** Warn on `:latest` or untagged images. Mutable tags enable silent supply-chain attacks and break rollback.

**Trigger:** `k8s.apply` action where any container image ends with `:latest` or has no tag.

**Hints:** Pin images to a specific version tag or SHA digest · Avoid :latest in production manifests

**Source:** kube-score `container-image-tag`

---

#### k8s.no_resource_limits

**Intent:** Warn on containers missing CPU or memory limits. Unbounded containers can OOMKill every co-located pod.

**Trigger:** `k8s.apply` action where any container is missing `resources.limits.cpu` or `resources.limits.memory`.

**Hints:** Set resources.limits.memory and resources.limits.cpu · Memory limits prevent OOMKill cascading

**Source:** kube-score `container-resources`

---

### ArgoCD

---

#### argocd.autosync_prod

**Intent:** Deny automated sync in production. Every Git commit should not immediately deploy without human review.

**Trigger:** `argocd.sync` action with `sync_policy.automated == true` when `argocd.autosync.deny_automated` param is `true`.

| Param | Default | Production |
|---|---|---|
| `argocd.autosync.deny_automated` | false | true |

**Hints:** Disable automated sync for production Applications · Use manual sync with approval gates

**Source:** ArgoCD Automated Sync documentation

---

#### argocd.wildcard_destination

**Intent:** Deny wildcard cluster or namespace targets in AppProjects. Wildcards allow any Application to deploy anywhere.

**Trigger:** `argocd.project` action where any destination has `namespace: "*"` or `server: "*"`.

**Hints:** Specify explicit namespace and server targets · Do not use the default AppProject for production

**Source:** ArgoCD Projects documentation

---

#### argocd.dangerous_sync_combo

**Intent:** Deny the combination of automated sync + prune + selfHeal in production. This creates a system that automatically deletes resources AND prevents human intervention.

**Trigger:** `argocd.sync` action with `sync_policy.automated.prune == true` and `sync_policy.automated.self_heal == true` when `argocd.dangerous_sync.deny_combo` param is `true`.

| Param | Default | Production |
|---|---|---|
| `argocd.dangerous_sync.deny_combo` | false | true |

**Hints:** Disable selfHeal or prune on production Applications with auto-sync · Use manual sync for production

**Source:** ArgoCD GitHub issue #14090, ArgoCD Sync documentation

---

### AWS S3

---

#### aws_s3.no_encryption

**Intent:** Deny S3 buckets without server-side encryption. SSE-S3 is zero-cost and zero-performance-impact.

**Trigger:** `terraform.plan` action with `resource_type: "aws_s3_bucket"` where `server_side_encryption.enabled` is not `true`.

**Hints:** Enable server-side encryption (SSE-S3 or SSE-KMS) · SSE-S3 is zero-cost and zero-performance-impact

**Source:** tfsec AVD-AWS-0088

---

#### aws_s3.no_versioning_prod

**Intent:** Deny versioning disabled on production S3 buckets. Without versioning, a single delete permanently destroys data.

**Trigger:** `terraform.plan` action with `resource_type: "aws_s3_bucket"` where `versioning.enabled` is not `true`, when `aws_s3.versioning.require` param is `true`.

| Param | Default | Production |
|---|---|---|
| `aws_s3.versioning.require` | false | true |

**Hints:** Enable versioning on production S3 buckets · Versioning converts destructive deletes into recoverable soft-deletes

**Source:** tfsec AVD-AWS-0090

---

### AWS IAM

---

#### aws_iam.wildcard_policy

**Intent:** Deny IAM policies with `Action: *` combined with `Resource: *` — functionally root for the AWS account.

**Trigger:** `terraform.plan` action with `iam_policy_statements` containing `effect: "Allow"`, `action: "*"`, and `resource: "*"` simultaneously.

**Hints:** Remove Action:\* and Resource:\* from IAM policy statements · Scope permissions to specific services and ARNs

**Source:** tfsec AVD-AWS-0057, Rhino Security Labs, Bishop Fox iam-vulnerable

---

#### aws_iam.wildcard_principal

**Intent:** Deny IAM trust policies with `Principal: *` or `Principal: {"AWS": "*"}` — allows any AWS account to assume the role.

**Trigger:** `terraform.plan` action with `trust_policy_statements` containing `effect: "Allow"` with wildcard principal.

**Hints:** Replace Principal:\* with specific AWS account IDs or service principals · Add Condition keys to restrict role assumption

**Source:** Datadog Security Labs, Hacking The Cloud

---

## 4. Operational Rules (5)

These rules provide operational guardrails and audit signals.

---

#### k8s.protected_namespace

Deny changes targeting restricted namespaces (default: `kube-system`) without `breakglass` tag.

| Param | Default |
|---|---|
| `k8s.namespaces.restricted` | ["kube-system"] |

---

#### ops.unapproved_change

Deny changes targeting protected namespaces (default: `prod`) without `change-approved` tag.

| Param | Default |
|---|---|
| `k8s.namespaces.protected` | ["prod"] |

---

#### ops.public_exposure

Deny `terraform.plan` actions where `publicly_exposed == true` without `approved_public` tag.

---

#### ops.autonomous_execution

Warn when an agent request arrives via MCP. Flags for human review.

---

#### ops.breakglass_used

Warn when any action carries the `breakglass` tag. Ensures audit trail.

---

## 5. Params Index

| Param Key | Used By | Type | Default | Env Override |
|---|---|---|---|---|
| `ops.mass_delete.max_deletes` | `ops.mass_delete` | number | 5 | production: 3 |
| `k8s.namespaces.restricted` | `k8s.protected_namespace` | list | ["kube-system"] | — |
| `k8s.namespaces.protected` | `ops.unapproved_change` | list | ["prod"] | — |
| `k8s.dangerous_capabilities.list` | `k8s.dangerous_capabilities` | list | ["SYS_ADMIN", "SYS_PTRACE", "NET_RAW"] | — |
| `terraform.sg_open_world.dangerous_ports` | `terraform.sg_open_world` | list | [22, 3389] | — |
| `argocd.autosync.deny_automated` | `argocd.autosync_prod` | boolean | false | production: true |
| `argocd.dangerous_sync.deny_combo` | `argocd.dangerous_sync_combo` | boolean | false | production: true |
| `aws_s3.versioning.require` | `aws_s3.no_versioning_prod` | boolean | false | production: true |

---

## 6. Authoring Guidelines

**Rule IDs:** `domain.invariant_name` — lowercase, dot-separated. Domain may use underscores for AWS-specific namespaces (e.g., `aws_iam.wildcard_policy`).

**Tunable values:** All thresholds and lists in `evidra/data/params/data.json`, never in rule bodies. Every param must define `by_env.default`.

**Environment in rules:** No environment names as string literals. Use `resolve_param` with `by_env` entries.

**Hints:** In `evidra/data/rule_hints/data.json`, keyed by canonical rule ID. 1–3 actionable strings per rule.
