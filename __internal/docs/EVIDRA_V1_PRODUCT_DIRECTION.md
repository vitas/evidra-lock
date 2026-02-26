# Evidra — Product Direction

**Updated:** 2026-02-26

---

## What Evidra Is

Evidra is a policy evaluation and evidence signing system for AI agent infrastructure operations. Before an AI agent executes `kubectl apply` or `terraform apply`, it calls Evidra. Evidra evaluates OPA policy, returns allow/deny with risk level and remediation hints, and produces a cryptographically verifiable evidence record.

Target user: DevOps engineer using AI coding assistants (Claude Code, Cursor, Copilot) for infrastructure work.

---

## What Changed Since v0.1

The project shifted from "local CLI tool" to "API-first with offline fallback":

- **Ed25519 signing** — designed for P0, not Phase 3. Every API response includes a signed evidence record verifiable offline with the public key.
- **Hosted API** — the primary interface, not an optional expansion. CLI and MCP are now API clients with local OPA fallback.
- **Input adapters** — external binaries in a separate repo (`evidra/adapters`), not built into core. Transform raw tool artifacts into structured ToolInvocation params.
- **Hybrid mode** — CLI and MCP resolve online/offline at startup, test reachability at call time. Configurable fallback policy.
- **23 rules** — the policy library expanded from 6 to 23 rules covering Kubernetes (CIS 5.2.x), Terraform, S3, IAM, and ArgoCD.

---

## Core Value Proposition

Evidra validates the outcome of infrastructure changes before they run.

It does not filter command strings. It evaluates structured plan/diff descriptions of intended changes, applies deterministic policy, and returns:

- Allow / Deny
- Risk level (low / medium / high)
- Rule IDs and human-readable reasons
- Actionable remediation hints
- Signed evidence record (online) or hash-linked evidence (offline)

---

## Not in Scope

- Compliance automation (SOC2, HIPAA, PCI)
- Approval workflow engines
- Governance dashboards (until API is proven)
- Generic DevSecOps platform features
- Runtime security or live infrastructure inspection

---

## Product Filter

> Does it strengthen deterministic validation of infrastructure outcomes?

If no — reject or postpone.

---

## Release Readiness (P0-API)

- [ ] API Phase 0 deployed and accessible at `evidra.rest`
- [ ] Hybrid mode in CLI + MCP
- [ ] Terraform adapter v0.1.0 released
- [ ] Architecture docs up to date
- [ ] README updated to show both online and offline workflows
- [ ] Dogfooding CI validates infrastructure PRs
