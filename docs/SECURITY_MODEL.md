# Security Model

## Design Philosophy

Evidra is a pre-execution validation layer. The policy baseline (`ops-v0.1`) contains 23 rules focused exclusively on catastrophic failure prevention: production namespace deletion, world-open security groups, wildcard IAM policies, privileged container escape.

### What Evidra does

- Evaluates infrastructure changes against deterministic policy before execution.
- Returns structured decisions with rule IDs, reasons, and actionable hints.
- Records every decision (allow and deny) in a tamper-evident evidence chain.

### Non-goals

- **Not a CIS compliance scanner.** Does not implement full CIS benchmarks or aim for checkbox coverage.
- **Not a replacement for tfsec, trivy, or checkov.** Those tools perform deep static analysis across hundreds of rules. Evidra evaluates a small, curated set of catastrophic guardrails.
- **Not a runtime security tool.** No runtime API calls, no cloud provider connections, no live infrastructure inspection.
- **Not an enforcement gateway.** Evidra validates and records — it does not execute commands or manage infrastructure directly.

---

## Deterministic Evaluation

All policy evaluation is local and deterministic:

- Input is static configuration data (Terraform plan JSON, Kubernetes manifests, ArgoCD sync policies).
- No network calls during evaluation. No external dependencies at runtime.
- Given the same input and policy bundle, the same decision is produced every time.
- The OPA engine is embedded in the binary. Parameters are resolved from the bundle's `data.json` using a `by_env` fallback chain (environment-specific → default).
- Evaluation works in air-gapped environments.

---

## Enforcement Model

Two modes are supported:

- **Enforce** (default): Deny decisions block the action. The AI agent receives a structured denial and cannot proceed without addressing the policy violation.
- **Observe** (`--observe`): Policy is evaluated and recorded, but denials are downgraded to advisories. Useful for rollout and tuning.

In both modes, every decision is recorded to the evidence store. Observe mode does not skip logging.

---

## Evidence Integrity

The evidence store at `~/.evidra/evidence` is append-only JSONL with hash-linked records:

- Each record includes `previous_hash` (linking to the prior record) and a self-verifying `hash`.
- The hash covers the canonical JSON representation of the record (excluding the `hash` field itself).
- Tampering with any record breaks the hash chain, detectable via `evidra evidence verify`.
- If evidence cannot be written, the validation pipeline returns an error — the caller cannot silently bypass logging.

Evidence records contain:

- Actor identity and origin (human, agent, system; cli, mcp).
- Tool, operation, and target parameters.
- Full policy decision (allow/deny, risk level, rule IDs, reasons, hints).
- Timestamps and event IDs.

---

## Known Limitations

- **Bypass risk:** Any execution path that skips `pkg/validate` / `evidra-mcp` is not covered. If an agent can execute commands without calling the `validate` tool, Evidra provides no protection for that path.
- **Host-level access:** An adversary with root access to the host can rewrite the evidence store directly. Mitigate by exporting evidence to an external system.
- **Static analysis only:** Evidra evaluates declared configuration, not runtime state. A Terraform plan that passes policy may still produce unexpected results due to provider behavior.

---

## Recommended Deployment

- Run `evidra-mcp` in an isolated runtime with network controls so only trusted clients can submit tool invocations.
- Restrict agent shells so they cannot bypass the MCP server or the offline `evidra validate` CLI.
- Export evidence segments to a hardened store for long-term auditing: `evidra evidence export`.
- Start with `--observe` mode to validate policy against real workloads before switching to enforce.
