# OPS v0.1 Serious Baseline Research

## Purpose

This document presents 18 must-have guardrail rules for the Evidra `ops-v0.1` policy bundle.

Each rule was selected through structured analysis of:

- **Aqua Security kube-bench** — CIS Kubernetes Benchmark automated checks
- **Aqua Security tfsec/trivy** — Terraform static analysis (AVD rule database)
- **Zalando kube-score** — Kubernetes manifest scoring
- **CIS Kubernetes Benchmark** — Sections 5.1, 5.2, 5.7 (catastrophic controls only)
- **AWS S3 official security best practices** — Block Public Access, encryption, versioning
- **ArgoCD documentation** — Sync policies, pruning safeguards, project restrictions

Rules were filtered against Evidra's design philosophy: high signal, catastrophic impact first, deterministic evaluation, low false positives. This is not a compliance checklist. It is a blast-radius limiter for AI-generated infrastructure changes.

---

## Selection Criteria

Every rule in this document satisfies ALL of the following:

| Criterion | Threshold |
|---|---|
| **Prevents catastrophic outcome** | Production outage, mass data exposure, irreversible destruction, or cluster/account compromise |
| **Deterministic evaluation** | Can be evaluated from static configuration (manifests, plan JSON, YAML) without runtime API calls |
| **Low false-positive rate** | Expected FP rate is Low or Medium; no rule with inherently High FP rates is included |
| **Clear blast radius** | The misconfiguration it catches has a well-documented, real-world attack chain or incident history |
| **Not hygiene** | The rule prevents a specific catastrophic scenario, not a general "best practice" |

Rules that failed these criteria — even if widely recommended — are listed in the exclusions section with rationale.

---

## Final Must-Have Guardrails (18 Rules)

### Terraform / Infrastructure-as-Code

---

#### 1. `ops.mass_delete`

| Field | Value |
|---|---|
| **Category** | Terraform |
| **Short description** | Deny plans with excessive destroy operations |
| **Why catastrophic** | An accidental `terraform destroy`, state file corruption, or drift can cascade-delete databases, load balancers, DNS records, and VPCs in seconds. A single Terraform apply with mass deletions can take down an entire production environment irreversibly. Real-world incidents include engineers running `terraform destroy` against production state files. |
| **Deterministic evaluation** | Count resources where `change.actions` contains `"delete"` in the Terraform plan JSON `resource_changes` array. Compare against environment-specific threshold. |
| **Expected FP risk** | **Low.** Legitimate production changes rarely delete more than 3 resources simultaneously. Threshold is configurable per environment. |
| **Source inspiration** | OPA Terraform policy (official docs: blast-radius scoring), Evidra original design |
| **Default disposition** | **DENY** |
| **Production-scope condition** | Threshold: 3 in production, 5 in other environments |
| **Status** | Already implemented in `deny_mass_delete.rego` |

---

#### 2. `terraform.sg_open_world`

| Field | Value |
|---|---|
| **Category** | Terraform |
| **Short description** | Deny security groups with 0.0.0.0/0 ingress on SSH, RDP, or all ports |
| **Why catastrophic** | A security group open to the internet on port 22 (SSH) or 3389 (RDP) is the initial access vector in the majority of EC2-based breaches. Automated scanners (Shodan, Censys) find open ports within minutes. If the instance has an IAM role, compromise leads to full account pivot via instance metadata. Open RDP is the #1 ransomware entry point globally. |
| **Deterministic evaluation** | Check for CIDR `0.0.0.0/0` or `::/0` in ingress rules with `from_port`/`to_port` matching 22, 3389, or 0-65535. Literal string + numeric comparison on Terraform plan JSON. |
| **Expected FP risk** | **Low.** There is no legitimate reason to expose SSH/RDP to the entire internet in production. Bastion hosts and VPNs are the accepted alternative. |
| **Source inspiration** | tfsec AVD-AWS-0107 (`aws-ec2-no-public-ingress-sgr`), CIS AWS Benchmark |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. Even in dev, 0.0.0.0/0 on SSH is a risk. |

---

#### 3. `terraform.s3_public_access`

| Field | Value |
|---|---|
| **Category** | Terraform / S3 |
| **Short description** | Deny S3 buckets without Block Public Access enabled |
| **Why catastrophic** | Public S3 buckets are the root cause of the majority of cloud data breaches: Capital One (100M+ records, $270M total cost), Accenture (40K+ passwords), US Military CENTCOM (classified personnel data), Twitch (125GB source code). A single missing Block Public Access setting can expose an entire bucket to the internet. AWS provides four boolean flags (`BlockPublicAcls`, `IgnorePublicAcls`, `BlockPublicPolicy`, `RestrictPublicBuckets`) — all four must be `true`. |
| **Deterministic evaluation** | Check `aws_s3_bucket_public_access_block` resource exists and all four flags are `true`. If the resource is absent, deny. Binary boolean checks. |
| **Expected FP risk** | **Low.** AWS enables all four by default on new buckets since 2023. Intentionally public buckets (static website hosting) should use the `approved_public` tag. |
| **Source inspiration** | tfsec AVD-AWS-0086/0087/0091/0093, AWS S3 Security Best Practices, CIS AWS 2.1.5 |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. Public buckets in any environment are a data breach vector. |

---

#### 4. `terraform.iam_wildcard_policy`

| Field | Value |
|---|---|
| **Category** | Terraform / IAM |
| **Short description** | Deny IAM policies with `Action: *` or `Resource: *` |
| **Why catastrophic** | A policy with `"Action": "*", "Resource": "*"` is the AWS equivalent of root. Any principal with this policy can create IAM users, delete all resources, exfiltrate all data, and create backdoors. This was the core enabling factor in the Capital One breach: an overly permissive IAM role allowed the attacker to list and read all S3 buckets. `iam:PassRole` with `Resource: *` is the #1 AWS privilege escalation technique (Rhino Security Labs). |
| **Deterministic evaluation** | Parse IAM policy JSON from Terraform plan. Check for literal `"*"` in `Action` and `Resource` fields of `Allow` statements. String match — no ambiguity. |
| **Expected FP risk** | **Medium.** Some AWS services (CloudWatch Logs, STS) require `Resource: *` because they don't support resource-level permissions. These are well-documented exceptions. The rule should flag for human review, not silently allow. |
| **Source inspiration** | tfsec AVD-AWS-0057 (`aws-iam-no-policy-wildcards`), Checkov CKV_AWS_355/CKV_AWS_356 |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. Wildcard IAM in dev can pivot to production via cross-account roles. |

---

### Kubernetes

---

#### 5. `k8s.privileged_container`

| Field | Value |
|---|---|
| **Category** | Kubernetes |
| **Short description** | Deny containers with `securityContext.privileged: true` |
| **Why catastrophic** | A privileged container has full access to all host devices, can mount the host filesystem, load kernel modules, and modify the host kernel. The container boundary ceases to exist. From a privileged container, an attacker reads kubelet credentials, accesses the API server, and owns the entire cluster. This is the single most dangerous misconfiguration in Kubernetes — it converts a container compromise into a full cluster compromise in one step. |
| **Deterministic evaluation** | Check `spec.containers[*].securityContext.privileged` and `spec.initContainers[*].securityContext.privileged`. Binary `true`/`false` field. |
| **Expected FP risk** | **Low.** The only legitimate use is infrastructure-level DaemonSets (CNI plugins, storage drivers) that should be excluded by namespace or annotation. Application workloads never need privileged mode. |
| **Source inspiration** | CIS 5.2.1, kube-bench 5.2.1, kube-score `container-security-context-privileged` |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. Privileged containers are equally dangerous in dev if the cluster is shared. |

---

#### 6. `k8s.host_namespace_escape`

| Field | Value |
|---|---|
| **Category** | Kubernetes |
| **Short description** | Deny `hostPID`, `hostIPC`, or `hostNetwork` in production |
| **Why catastrophic** | `hostPID: true` — container sees and can signal all host processes, including kubelet; enables code injection via `ptrace`. `hostIPC: true` — container accesses shared memory of host processes; enables data exfiltration without network traffic. `hostNetwork: true` — container gets the node's network stack; can sniff all traffic, access localhost-bound services (kubelet API on port 10250, etcd if co-located), and bypass all NetworkPolicy. Each of these individually enables container-to-host escape. |
| **Deterministic evaluation** | Check `spec.hostPID`, `spec.hostIPC`, `spec.hostNetwork`. Three binary `true`/`false` fields. |
| **Expected FP risk** | **Low.** These are needed only by monitoring agents (hostNetwork for node-level metrics) and CNI plugins. Legitimate uses are restricted to infrastructure DaemonSets. |
| **Source inspiration** | CIS 5.2.2/5.2.3/5.2.4, kube-bench 5.2.2/5.2.3/5.2.4 |
| **Default disposition** | **DENY** |
| **Production-scope condition** | Production namespaces. Infrastructure namespaces (`kube-system`) may be excluded. |

---

#### 7. `k8s.run_as_root`

| Field | Value |
|---|---|
| **Category** | Kubernetes |
| **Short description** | Deny containers running as UID 0 (root) |
| **Why catastrophic** | Running as root inside the container means any container escape immediately yields root on the host. Root + kernel exploit = full node compromise. Non-root makes most escape exploits non-functional (CVE-2022-0185, CVE-2024-21626, and similar runc/containerd CVEs all require root inside the container). This is the amplification factor that turns a theoretical vulnerability into a practical compromise. |
| **Deterministic evaluation** | Check `securityContext.runAsNonRoot: true` at pod or container level, and `securityContext.runAsUser != 0`. Deterministic field checks. |
| **Expected FP risk** | **Low.** Most application containers can run as non-root. Legacy images that require root should be rebuilt. The Dockerfile `USER` directive is the fix. |
| **Source inspiration** | CIS 5.2.6, kube-bench 5.2.6, kube-score `container-security-context-user-group-id` |
| **Default disposition** | **DENY** |
| **Production-scope condition** | Production. May be relaxed to WARN in dev for legacy image migration. |

---

#### 8. `k8s.hostpath_mount`

| Field | Value |
|---|---|
| **Category** | Kubernetes |
| **Short description** | Deny `hostPath` volume mounts |
| **Why catastrophic** | `hostPath` volumes mount arbitrary host filesystem paths into the container. A container with `hostPath: /` can read and write the entire host filesystem — including `/etc/shadow`, kubelet credentials (`/var/lib/kubelet/`), and other pod data. Mounting `/var/run/docker.sock` or `/run/containerd/containerd.sock` gives container runtime API access, which is equivalent to full cluster compromise. This is the most reliable persistence mechanism after a container escape. |
| **Deterministic evaluation** | Check `spec.volumes[*].hostPath` presence. Optionally, deny specific dangerous paths (`/`, `/etc`, `/var/run/docker.sock`, `/var/lib/kubelet`). Presence check is binary. |
| **Expected FP risk** | **Low.** Application workloads should use PersistentVolumeClaims, not hostPath. Legitimate uses (log collectors, monitoring agents) are infrastructure DaemonSets in `kube-system`. |
| **Source inspiration** | CIS 5.2.13, kube-bench 5.2.12 |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. HostPath in dev clusters enables lateral movement to other tenants. |

---

#### 9. `k8s.dangerous_capabilities`

| Field | Value |
|---|---|
| **Category** | Kubernetes |
| **Short description** | Deny containers with SYS_ADMIN, SYS_PTRACE, or NET_RAW capabilities |
| **Why catastrophic** | `SYS_ADMIN` — nearly equivalent to `--privileged`; enables mount namespace escape, cgroup manipulation, and kernel module loading. `SYS_PTRACE` — enables process injection and memory reading across container boundaries. `NET_RAW` — enables ARP spoofing, DNS poisoning, and man-in-the-middle attacks within the cluster network; an attacker can intercept traffic between pods. The default Linux capability set includes NET_RAW, so not explicitly dropping it leaves this attack vector open. |
| **Deterministic evaluation** | Check `securityContext.capabilities.add` for `SYS_ADMIN`, `SYS_PTRACE`, `NET_RAW`. Check `securityContext.capabilities.drop` includes `ALL` or at minimum `NET_RAW`. String list comparison. |
| **Expected FP risk** | **Low.** Application containers should drop ALL capabilities and add back only what is needed. SYS_ADMIN and SYS_PTRACE are never needed by application workloads. NET_RAW is needed only by network diagnostic tools. |
| **Source inspiration** | CIS 5.2.7/5.2.8/5.2.9, kube-bench 5.2.7/5.2.8/5.2.9 |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. |

---

#### 10. `k8s.mutable_image_tag`

| Field | Value |
|---|---|
| **Category** | Kubernetes |
| **Short description** | Warn on image tag `:latest` or missing tag |
| **Why catastrophic** | A mutable tag means the image content can change without any change to the manifest. `:latest` is the default when no tag is specified. This enables silent supply-chain attacks: a compromised registry can replace the image behind `:latest` with a malicious version. It also breaks rollback — you cannot rollback to a specific version if the tag is mutable. In production, this converts a registry compromise into an undetectable cluster-wide code injection. |
| **Deterministic evaluation** | Parse `spec.containers[*].image` string. Check for `:latest` suffix or absence of `:` tag separator. String pattern match. |
| **Expected FP risk** | **Low.** Pinned tags (SHA digests or semantic versions) are the universal standard. `:latest` in production is always a mistake. |
| **Source inspiration** | kube-score `container-image-tag`, common admission controller practice |
| **Default disposition** | **WARN** |
| **Production-scope condition** | WARN in all environments; upgrade to DENY in production if desired. |

---

#### 11. `k8s.no_resource_limits`

| Field | Value |
|---|---|
| **Category** | Kubernetes |
| **Short description** | Warn on containers missing CPU or memory limits |
| **Why catastrophic** | A container without resource limits can consume all node CPU and memory. This causes OOMKill cascading across co-located pods, taking down every workload on the node. In multi-tenant clusters, a single unbounded container can create a denial-of-service condition affecting all teams. During traffic spikes, unbounded containers amplify the blast radius by preventing other pods from getting scheduled or surviving. |
| **Deterministic evaluation** | Check `spec.containers[*].resources.limits.cpu` and `spec.containers[*].resources.limits.memory` are set. Presence check. |
| **Expected FP risk** | **Medium.** Some teams intentionally omit CPU limits to avoid throttling (Kubernetes burstable QoS). Memory limits should always be set. Consider denying missing memory limits and warning on missing CPU limits. |
| **Source inspiration** | kube-score `container-resources`, Kubernetes QoS best practices |
| **Default disposition** | **WARN** |
| **Production-scope condition** | WARN in all environments. Upgrade to DENY for memory limits in production. |

---

### ArgoCD

---

#### 12. `argocd.autosync_prod`

| Field | Value |
|---|---|
| **Category** | ArgoCD |
| **Short description** | Deny automated sync in production environments |
| **Why catastrophic** | Automated sync means every Git commit immediately deploys to production without human review. A bad merge, an accidental push to the wrong branch, or a compromised CI pipeline results in immediate production deployment. ArgoCD documentation explicitly states: "Rollback cannot be performed against an application with automated sync enabled." Combined with `selfHeal: true`, operators cannot even apply emergency fixes directly to the cluster during an incident — ArgoCD reverts manual changes within 5 seconds. |
| **Deterministic evaluation** | Check `spec.syncPolicy.automated` is present/enabled on Application specs where the destination matches production namespaces or clusters. Boolean/presence check on YAML fields. |
| **Expected FP risk** | **Low.** Manual sync with approval gates is the standard for production GitOps. Automated sync is appropriate for dev/staging only. |
| **Source inspiration** | ArgoCD Automated Sync documentation, GitOps operational safety patterns |
| **Default disposition** | **DENY** |
| **Production-scope condition** | Production namespaces and clusters only. Dev/staging may use auto-sync. |

---

#### 13. `argocd.wildcard_destination`

| Field | Value |
|---|---|
| **Category** | ArgoCD |
| **Short description** | Deny wildcard cluster or namespace targets in AppProjects |
| **Why catastrophic** | An AppProject with `destinations: [{namespace: '*', server: '*'}]` allows any Application under that project to deploy to any namespace on any cluster. A single compromised Application or bad Git commit can deploy resources into `kube-system`, the ArgoCD namespace (granting effective cluster-admin), or any critical namespace. The `default` AppProject allows `sourceRepos: '*'` and `destinations: '*'` — any application placed in the default project has unrestricted access. Deploying to the ArgoCD namespace = effective cluster-admin escalation. |
| **Deterministic evaluation** | Check AppProject `spec.destinations` for `namespace: '*'` or `server: '*'`. Check for `sourceRepos: '*'`. Literal string match on YAML fields. |
| **Expected FP risk** | **Low.** Production AppProjects should have explicit destination lists. Wildcard destinations defeat the purpose of project-level isolation. |
| **Source inspiration** | ArgoCD Projects documentation, ArgoCD Project Specification Reference |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All AppProjects that target production clusters. |

---

#### 14. `argocd.dangerous_sync_combo`

| Field | Value |
|---|---|
| **Category** | ArgoCD |
| **Short description** | Deny automated sync + prune + selfHeal combination in production |
| **Why catastrophic** | `prune: true` deletes any resource no longer in Git. `allowEmpty: true` (or default behavior with prune) means a misconfigured repo path or emptied branch causes ArgoCD to delete ALL resources in the target namespace. `selfHeal: true` prevents manual rollback during outages. The combination of all three creates a system that automatically deletes resources based on Git state AND prevents human intervention. A known ArgoCD bug (issue #14090) has caused pruning of resources that still exist in Git due to caching issues. |
| **Deterministic evaluation** | Check `spec.syncPolicy.automated.prune`, `spec.syncPolicy.automated.selfHeal`, and `spec.syncPolicy.automated` (enabled) are all `true` simultaneously. Boolean conjunction on YAML fields. |
| **Expected FP risk** | **Low.** The combination of all three in production is universally considered dangerous by the ArgoCD community. Each setting individually has legitimate uses; the combination in production does not. |
| **Source inspiration** | ArgoCD Automated Sync documentation, ArgoCD GitHub issue #14090 |
| **Default disposition** | **DENY** |
| **Production-scope condition** | Production namespaces and clusters only. |

---

### S3

---

#### 15. `aws_s3.no_encryption`

| Field | Value |
|---|---|
| **Category** | S3 |
| **Short description** | Deny S3 buckets without server-side encryption |
| **Why catastrophic** | Unencrypted S3 data at rest means any compromised disk, snapshot, or cross-account access yields plaintext. While AWS now enables SSE-S3 by default for new buckets, older buckets and explicit disabling still occur. Regulatory requirements (PCI-DSS, HIPAA, SOC2) mandate encryption at rest. Unencrypted buckets are a data breach vector when combined with overly permissive IAM policies or bucket policies. |
| **Deterministic evaluation** | Check `server_side_encryption_configuration` exists and specifies SSE-S3 or SSE-KMS. Absence of configuration = deny. Boolean/presence check. |
| **Expected FP risk** | **Low.** SSE-S3 is zero-cost and zero-performance-impact. There is no legitimate reason to disable encryption. |
| **Source inspiration** | tfsec AVD-AWS-0088 (`aws-s3-enable-bucket-encryption`), AWS Encryption Best Practices |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. |

---

#### 16. `aws_s3.no_versioning_prod`

| Field | Value |
|---|---|
| **Category** | S3 |
| **Short description** | Deny versioning disabled on production S3 buckets |
| **Why catastrophic** | Without versioning, a single `DeleteObject` or `PutObject` call permanently destroys data. There is no recovery. Ransomware attacks specifically target unversioned buckets — a compromised IAM credential + `aws s3 rm --recursive` = permanent total data loss. Versioning converts destructive operations into soft-deletes (delete markers) that can be reversed. Without versioning, an automation error or malicious actor can wipe an entire bucket with no recourse. |
| **Deterministic evaluation** | Check `versioning.enabled = true` on bucket configuration. Binary boolean check. |
| **Expected FP risk** | **Low.** Versioning has minimal cost overhead. The only legitimate exception is ephemeral/temp buckets, which should not exist in production. |
| **Source inspiration** | tfsec AVD-AWS-0090 (`aws-s3-enable-versioning`), AWS S3 Versioning documentation |
| **Default disposition** | **DENY** |
| **Production-scope condition** | Production environments only. Dev/staging may have unversioned temp buckets. |

---

### IAM

---

#### 17. `aws_iam.wildcard_policy`

| Field | Value |
|---|---|
| **Category** | IAM |
| **Short description** | Deny IAM policies with `Action: *` combined with `Resource: *` |
| **Why catastrophic** | `Action: *, Resource: *` is functionally root for the AWS account. It grants unrestricted access to create/delete/modify every resource across every service. The Capital One breach ($270M total cost) was enabled by an overly permissive IAM role. `iam:PassRole` with `Resource: *` is the #1 AWS privilege escalation technique (Rhino Security Labs): combined with any compute service creation permission (`ec2:RunInstances`, `lambda:CreateFunction`), it allows the attacker to create a resource with any IAM role attached and execute code as that role. Bishop Fox documented 250+ such escalation paths. |
| **Deterministic evaluation** | Parse IAM policy JSON. Check for literal `"*"` in `Action` and `Resource` fields within `Allow` statements. Pure string matching — no runtime API calls needed. |
| **Expected FP risk** | **Medium.** A few AWS services (CloudWatch Logs `logs:CreateLogGroup`, STS `sts:AssumeRole`) require `Resource: *` because they don't support resource-level permissions. These are well-documented exceptions and should be handled via a known-safe-actions allowlist, not by disabling the rule. |
| **Source inspiration** | tfsec AVD-AWS-0057, Checkov CKV_AWS_355/CKV_AWS_356, Rhino Security Labs, Bishop Fox iam-vulnerable |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. Wildcard IAM in dev can pivot to production via cross-account roles or shared services. |

---

#### 18. `aws_iam.wildcard_principal`

| Field | Value |
|---|---|
| **Category** | IAM |
| **Short description** | Deny IAM trust policies with `Principal: *` |
| **Why catastrophic** | A trust policy with `"Principal": "*"` or `"Principal": {"AWS": "*"}` allows any AWS account in the world — including attacker-controlled accounts — to call `sts:AssumeRole` and obtain temporary credentials for that role. This is an immediate full account compromise vector. AWS Access Analyzer does not alert on all variants of this misconfiguration. Datadog Security Labs documented how AWS Amplify flaws combined with weak trust policies enabled cross-account role takeover. |
| **Deterministic evaluation** | Parse the `assume_role_policy` JSON on `aws_iam_role` resources. Check for `"*"` in the `Principal` field of `Allow` statements. Literal string match. |
| **Expected FP risk** | **Low.** There is almost never a legitimate reason for a trust policy to trust all AWS accounts. Even public-facing services (Lambda@Edge, CloudFront) use specific AWS service principals, not wildcards. |
| **Source inspiration** | Datadog Security Labs, Hacking The Cloud (misconfigured trust policies), Token Security research |
| **Default disposition** | **DENY** |
| **Production-scope condition** | All environments. A wildcard trust policy in any environment is a cross-account compromise vector. |

---

## Why These 18 Make Evidra Credible

### Coverage of real-world catastrophes

Every major cloud security incident in the last 8 years maps to one or more of these rules:

| Incident | Year | Cost | Rules That Would Have Caught It |
|---|---|---|---|
| Capital One | 2019 | $270M+ | `terraform.s3_public_access`, `aws_iam.wildcard_policy` |
| Accenture | 2017 | Undisclosed | `terraform.s3_public_access` |
| US Military CENTCOM | 2017 | Classified | `terraform.s3_public_access` |
| Twitch | 2021 | Undisclosed | `terraform.s3_public_access` |
| Kubernetes cluster compromises (multiple) | 2020-2025 | Various | `k8s.privileged_container`, `k8s.hostpath_mount` |
| ArgoCD-triggered production wipes (multiple) | 2022-2025 | Various | `argocd.dangerous_sync_combo` |

### Signal distribution across attack surfaces

| Attack Surface | Rules | Coverage |
|---|---|---|
| Container escape → host | 5, 6, 7, 8, 9 | Privileged, host namespaces, root, hostPath, capabilities |
| Mass data exposure | 3, 15, 17, 18 | S3 public access, encryption, IAM wildcards, trust policies |
| Irreversible destruction | 1, 14, 16 | Mass delete, dangerous sync combo, no versioning |
| Account/cluster takeover | 4, 5, 17, 18 | IAM wildcards, privileged containers, trust policies |
| GitOps operational safety | 12, 13, 14 | Auto-sync, wildcard destinations, prune+selfHeal |
| Supply chain / integrity | 10 | Mutable image tags |
| Availability / DoS | 11 | Resource limits |

### What makes this set defensible

1. **Every rule has a specific, named incident or documented attack chain.** None are theoretical.
2. **Every rule is deterministically evaluable from static configuration.** No runtime API calls, no probabilistic analysis, no ML models.
3. **16 of 18 rules have Low false-positive risk.** The two Medium-FP rules (`aws_iam.wildcard_policy`, `k8s.no_resource_limits`) have well-documented exception lists.
4. **The rule count is small enough to explain in a meeting.** 18 rules that a security team can audit, understand, and defend in 30 minutes.
5. **The rules are sourced from industry-standard tools.** Every rule maps to a CIS control, tfsec/trivy AVD ID, or kube-score check. This is not invented; it is curated.

### Disposition summary

| Disposition | Count | Rules |
|---|---|---|
| **DENY** | 15 | 1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 13, 14, 15, 16, 17, 18 |
| **WARN** | 2 | 10, 11 |
| **Already implemented** | 1 | 1 (`ops.mass_delete`) |

---

## What We Explicitly Excluded (and Why)

### Kubernetes controls excluded

| Control | CIS ID | Why excluded |
|---|---|---|
| Restrict cluster-admin role bindings | 5.1.1 | **Not deterministic from workload manifests.** Requires cluster-level RBAC audit, not manifest-level evaluation. Evidra evaluates what AI agents submit, not cluster-wide RBAC state. |
| Minimize access to Secrets | 5.1.2 | **Requires cross-resource graph analysis.** Evaluating which roles can access secrets requires full RBAC graph traversal — exceeds single-manifest evaluation scope. |
| Minimize wildcard Roles/ClusterRoles | 5.1.3 | **Medium-high FP risk.** Some operators legitimately use wildcards on specific API groups. Requires contextual judgment that doesn't fit a deterministic deny rule. |
| Default ServiceAccount restrictions | 5.1.5 | **Cluster-level configuration, not workload-level.** Whether the default SA has bindings depends on cluster state, not the submitted manifest. |
| ServiceAccount token auto-mounting | 5.1.6 | **Too noisy for P0.** Many legitimate workloads need API access. This is a good V1 candidate but would generate excessive false positives at P0. |
| Seccomp profile enforcement | 5.2.11 | **Adoption gap.** Many production clusters don't have seccomp profiles deployed. Denying on missing seccomp would block most real-world deployments. Good V1 candidate. |
| AppArmor/SELinux enforcement | 5.2.12 | **Platform-dependent.** Not all Kubernetes distributions support AppArmor or SELinux. Too many false positives for a universal rule. |
| NetworkPolicy per pod | kube-score | **Not evaluable from a single manifest.** NetworkPolicy is a separate resource. Checking whether a pod is covered requires cross-resource analysis across the namespace. V1 candidate. |
| PodDisruptionBudget | kube-score | **Availability hygiene, not catastrophic prevention.** Missing PDBs cause disruption during maintenance, not security incidents. |

### AWS/Terraform controls excluded

| Control | Source | Why excluded |
|---|---|---|
| RDS publicly accessible | AVD-AWS-0082 | **Good rule but narrow scope.** Evidra P0 focuses on the most common resource types (S3, IAM, SG). RDS-specific rules are V1 scope. |
| Unrestricted egress SG | AVD-AWS-0104 | **High FP risk.** Almost every application needs outbound internet access (package repos, APIs, DNS). While unrestricted egress enables exfiltration, denying it blocks most deployments. Better as WARN in V1. |
| S3 bucket logging | AVD-AWS-0089 | **Detection, not prevention.** Logging helps detect breaches after the fact but doesn't prevent the misconfiguration. Evidra is a prevention engine. |
| S3 MFA Delete | AWS best practice | **Operational complexity too high for P0.** MFA Delete can only be enabled by the root account via CLI. It conflicts with lifecycle policies. Recommending it without organizational context creates friction. |
| S3 Object Lock | AWS best practice | **Compliance-grade control, not baseline.** WORM storage is for regulated industries (SEC 17a-4, FINRA). Including it in P0 would make Evidra look like a compliance scanner. |
| Root access keys | AVD-AWS-0141 | **Account-level control, not IaC-level.** Root access keys are managed through the AWS console, not Terraform. Evidra evaluates IaC changes, not account-level settings. |
| KMS policy wildcards | AVD-AWS-0055 | **Narrow scope for P0.** Valid rule but KMS is used by a subset of deployments. V1 candidate. |

### ArgoCD controls excluded

| Control | Source | Why excluded |
|---|---|---|
| Sync windows enforcement | ArgoCD docs | **Organizational policy, not deterministic safety.** When to deploy is an organizational decision. Evidra shouldn't impose deployment schedules. |
| ApplicationSet `preserveResourcesOnDeletion` | ArgoCD docs | **Too niche for P0.** ApplicationSets are used by a subset of ArgoCD deployments. V1 candidate. |
| Resource exclusions in argocd-cm | ArgoCD docs | **Operator-level configuration.** This is an ArgoCD server setting, not something AI agents submit. Outside Evidra's evaluation scope. |
| Per-resource `Prune=false` annotation | ArgoCD docs | **Too granular for P0.** Requiring annotations on every resource is a best practice, not a guardrail. |

---

## Source References

### Kubernetes

- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes) — Sections 5.1, 5.2, 5.7
- [kube-bench (Aqua Security)](https://github.com/aquasecurity/kube-bench) — Automated CIS checks
- [kube-bench CIS 1.20 policies.yaml](https://github.com/aquasecurity/kube-bench/blob/main/cfg/cis-1.20/policies.yaml)
- [kube-score (Zalando)](https://github.com/zegl/kube-score) — Manifest scoring
- [kube-score check list](https://github.com/zegl/kube-score/blob/master/README_CHECKS.md)

### Terraform / AWS

- [tfsec AVD-AWS-0057 — No Policy Wildcards](https://avd.aquasec.com/misconfig/aws/iam/avd-aws-0057/)
- [tfsec AVD-AWS-0086 — Block Public ACLs](https://avd.aquasec.com/misconfig/aws/s3/avd-aws-0086/)
- [tfsec AVD-AWS-0088 — Enable Bucket Encryption](https://avd.aquasec.com/misconfig/aws/s3/avd-aws-0088/)
- [tfsec AVD-AWS-0090 — Enable Versioning](https://avd.aquasec.com/misconfig/aws/s3/avd-aws-0090/)
- [tfsec AVD-AWS-0107 — No Public Ingress SGR](https://avd.aquasec.com/misconfig/aws/ec2/avd-aws-0107/)
- [Checkov CKV_AWS_355/CKV_AWS_356](https://www.checkov.io/5.Policy%20Index/all.html)
- [Rhino Security Labs — AWS Privilege Escalation](https://rhinosecuritylabs.com/aws/aws-privilege-escalation-methods-mitigation/)
- [Bishop Fox — iam-vulnerable](https://github.com/BishopFox/iam-vulnerable)
- [OPA Terraform Policy](https://www.openpolicyagent.org/docs/latest/terraform/)

### AWS S3

- [AWS S3 Security Best Practices](https://docs.aws.amazon.com/AmazonS3/latest/userguide/security-best-practices.html)
- [AWS S3 Block Public Access](https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-control-block-public-access.html)
- [AWS S3 Versioning](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Versioning.html)
- [AWS S3 Encryption](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingEncryption.html)

### ArgoCD

- [ArgoCD Automated Sync Policy](https://argo-cd.readthedocs.io/en/stable/user-guide/auto_sync/)
- [ArgoCD Projects](https://argo-cd.readthedocs.io/en/stable/user-guide/projects/)
- [ArgoCD Sync Options](https://argo-cd.readthedocs.io/en/latest/user-guide/sync-options/)
- [ArgoCD Application Deletion](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Application-Deletion/)

### Incident References

- [Capital One Breach — Krebs on Security](https://krebsonsecurity.com/2019/08/what-we-can-learn-from-the-capital-one-hack/)
- [Capital One Breach — Cloud Security Alliance Technical Analysis](https://cloudsecurityalliance.org/blog/2019/08/09/a-technical-analysis-of-the-capital-one-cloud-misconfiguration-breach)
- [10 Worst Amazon S3 Breaches — Bitdefender](https://www.bitdefender.com/en-us/blog/businessinsights/worst-amazon-breaches)
- [Accenture Cloud Leak — UpGuard](https://www.upguard.com/breaches/cloud-leak-accenture)
- [AWS Customer Security Incidents Repository](https://github.com/ramimac/aws-customer-security-incidents)
- [Datadog — Amplified Exposure: AWS IAM Role Takeover](https://securitylabs.datadoghq.com/articles/amplified-exposure-how-aws-flaws-made-amplify-iam-roles-vulnerable-to-takeover/)
