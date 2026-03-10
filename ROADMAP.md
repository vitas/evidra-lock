# Roadmap

Current release: v0.2.x
Last updated: 2026-03

---

## Current State (v0.2.x)

Evidra-Lock ships two protection levels:

**ops** (default) — kill-switch guardrails plus curated ops rules
covering K8s workloads, Terraform plan metadata, ArgoCD sync safety,
S3 public access, IAM wildcard detection, and open security groups.

**baseline** — kill-switch only. Blocks destructive operations with
missing context and unknown tools. No opinion on configuration.

### What works well
- Fail-closed evaluation for kubectl, terraform, helm, argocd
- Evidence chain (SHA-256 hash-linked, append-only)
- MCP integration (Claude Code, any MCP-compatible agent)
- CLI validation for CI pipelines
- Hosted MCP endpoint (api.evidra.rest)

### Known limitations
- **Input depends on agent accuracy.** Policy evaluation is
  deterministic (OPA, no LLM), but the agent constructs the payload.
  If the agent omits or hallucinates fields, evaluation is correct
  but on incomplete input. The kill-switch layer mitigates this
  (missing fields → deny), but deep policy rules may not fire on
  fields the agent didn't provide.
- **Terraform deep extraction not yet in CI adapter.** The v1
  adapter extracts plan metadata (counts, types, addresses). Rules
  that inspect security group CIDRs, IAM statements, or S3 config
  require the agent to extract these fields (MCP mode) or the
  planned v2 adapter.
- **No pre-execution manifest parsing.** For K8s workloads, the
  Rego canonicalizer handles native manifests. For Helm/ArgoCD,
  the agent provides extracted payload fields. v0.3.0 adds
  Go-side domain adapters that validate and extract from raw input.

---

## v0.3.0 — Domain Adapters (Engine v3)

**Goal:** Per-tool domain adapters that own canonicalization, intent
extraction, and missing-data detection. Eliminates agent-constructed
payload as the sole input path for non-K8s tools.

### What ships
- Go adapter registry with Resolve → Canonicalize → IntentKey flow
- **kubectl** adapter: K8s workload canonicalization stays in Rego
  (proven); intent extraction moves to Go (reads normalized output)
- **Helm** adapter: Go canonicalization (release, chart, values hash)
- **ArgoCD** adapter: Go canonicalization (app identity, sync params)
- **Terraform** adapter: passthrough + intent extraction
- Deny-loop prevention (stop_after_deny) using adapter-based intent keys
- Rego policy bundle reorganized into per-domain structure

### What this fixes
- Intent identity is derived from normalized (post-policy) output,
  not raw agent input — eliminates drift between "what policy sees"
  and "what deny-cache sees"
- Helm/ArgoCD payloads validated structurally before OPA evaluation
- Adapter telemetry in decision output (domain, mode, eligibility)

### Design reference
- [ENGINE_V3_DOMAIN_ADAPTERS.md](docs/ENGINE_V3_DOMAIN_ADAPTERS.md)

---

## v0.4.0 — DevSec Pack

**Goal:** A dedicated policy pack focused on container and cloud
security hardening. Complements the ops pack (catastrophic-only) with
security-focused rules that catch misconfigurations before deployment.

### Planned rule categories

**Container security (K8s)**
- Disallow containers running as UID 0 (explicit runAsUser: 0)
- Require readOnlyRootFilesystem
- Require resource limits (CPU + memory) on all containers
- Disallow privilege escalation (allowPrivilegeEscalation: true)
- Require security context on every container (no implicit defaults)
- Disallow hostPort bindings
- Require non-root user (runAsNonRoot: true)

**Image provenance**
- Deny images without digest (tag-only references)
- Deny images from untrusted registries (configurable allowlist)
- Deny `latest` tag (already `warn` in ops; `deny` in devsec)

**Network security (K8s)**
- Deny Services of type LoadBalancer without source range restriction
- Deny Ingress without TLS (when TLS resources are available)

**RBAC (K8s)**
- Deny ClusterRoleBindings to `cluster-admin`
- Deny wildcard verbs/resources in Roles

**Terraform / Cloud**
- Deny security groups with 0.0.0.0/0 on non-HTTP ports (extends ops rule)
- Deny IAM policies with `*` action on `*` resource
- Deny S3 buckets without versioning in production
- Deny unencrypted S3 buckets in production

### Policy pack design
- Ships as `devsec-v0.1` bundle alongside `ops-v0.1`
- Composable: teams can use ops + devsec together, or devsec alone
- Same flat action format — devsec rules read the same normalized
  fields that ops rules read
- Configurable per-environment (deny in prod, warn in dev)

### Depends on
- v0.3.0 domain adapters (some rules need K8s non-workload
  canonicalization: Service, Ingress, ClusterRoleBinding)
- adapter-terraform v2 (deep extraction for cloud security rules)

---

## v0.5.0+ — Future

Items on the radar, not yet committed:

- **Crossplane / CloudFormation adapters** — when demand exists
- **adapter-terraform v2** — deep extraction (SG rules, IAM
  statements, S3 config) from `terraform show -json` output
- **Execution wrapping** — evidra wraps kubectl/terraform/helm
  execution, enforcing validate-before-execute at process level
  (currently validate is called by convention, not enforcement)
- **Multi-cluster context** — cluster identity in evaluation
- **Policy marketplace** — community-contributed rule packs
- **Admission controller mode** — ValidatingWebhookConfiguration
  for teams that want Evidra-Lock in-cluster (complement to pre-execution)

---

## Non-goals

- **Compliance scanner.** Evidra-Lock checks actions, not cluster state.
  Use Trivy, Kubescape, or Prowler for compliance scanning.
- **Full admission controller replacement.** Evidra-Lock runs before
  execution; admission controllers run at deploy time. Keep both.
- **Natural language policy.** Policies are OPA/Rego. Deterministic,
  auditable, version-controlled. No prompt-based rules.
- **Runtime monitoring.** Evidra-Lock evaluates pre-execution intent.
  Use Falco or Tetragon for runtime security.
