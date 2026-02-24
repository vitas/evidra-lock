# Skill: Deterministic Change Gate (Evidra)

## Summary

This backlog item defines an AI Skill that uses Evidra MCP server as a deterministic policy gate and evidence backend before any change execution step.  
The Skill validates planned Terraform, Kubernetes, or generic changesets and returns a structured decision (`allow`, `deny`, or `observe`) with violations and an `evidence_id`.  
The primary purpose is safety and auditability: if decision is `deny`, execution must stop; if `allow` or `observe`, execution can proceed based on caller policy.  
The Skill is intended to be framework-agnostic so it can be used by Codex, Gemini clients (direct or bridged MCP), Claude Desktop, and CI pipelines.

## User Stories

- As an **agent developer**, I want a single validation capability that returns deterministic decisions so that I can enforce safe behavior in autonomous workflows.
- As a **DevOps engineer**, I want planned infrastructure changes checked before execution so that risky changes are blocked consistently.
- As a **Security/Compliance engineer**, I want every decision to produce a retrievable evidence record so that audits are traceable and reproducible.
- As an **SRE/on-call engineer**, I want clear deny reasons and remediation hints so that incidents caused by unsafe change plans are reduced.
- As a **Platform engineer**, I want one reusable gate for multiple clients and pipelines so that policy enforcement is centralized.
- As an **Auditor**, I want to verify decision integrity and chain consistency so that evidence cannot be silently tampered with.

## Explicit Non-goals

- No execution/apply of infrastructure changes.
- No cloud credential management.
- No policy authoring UI in v1.
- No remote evidence synchronization in v1 (future scope).
- No replacement of CI/CD system orchestration.

## Interfaces

### A) `validate_plan`

**Inputs**
- `kind`: `terraform | k8s | generic`
- `payload`: JSON object or file reference
- `profile`: string (example: `ops-v0.1`)
- `context`: environment, actor, ticket_id, dry_run, source, scenario_id, and related metadata

**Outputs**
- `decision`: `allow | deny | observe`
- `violations`: array of `{ rule_id, severity, message, hint? }`
- `evidence_id`: string
- `summary`: structured object (risk level, primary reason, counts, decision metadata)

### B) `get_evidence`

**Input**
- `evidence_id`

**Output**
- evidence event record
- hash-chain metadata (or references to verify chain integrity)

### C) `verify_evidence_chain` (optional MVP+)

**Input**
- evidence store path or bounded range selection

**Output**
- `ok | fail` with diagnostics

## Integration Points

### 1) MCP clients (Codex, Gemini, Claude Desktop)

**Who configures it**
- Agent/platform owner integrating MCP server definitions.

**Minimal required configuration**
- MCP server command: `evidra-mcp`
- Required args: `--bundle`
- Optional args: `--evidence-store` (or alias `--evidence-dir`), `--environment`, `--observe`
- Optional env: `EVIDRA_BUNDLE_PATH`, `EVIDRA_EVIDENCE_DIR`, `EVIDRA_ENVIRONMENT`

**Runtime data flow**
- Client sends `validate` tool call with action context.
- Skill maps input into Evidra-compatible invocation and calls MCP.
- Skill returns decision + violations + evidence_id to client logic.
- Client continues or aborts based on decision policy.

### 2) CI systems (GitHub Actions / GitLab CI)

**Who configures it**
- DevOps/platform team owning pipeline templates.

**Minimal required configuration**
- Start `evidra-mcp` as pipeline service/process OR call `evidra validate` directly for file-based checks.
- Ensure policy/data paths are pinned in pipeline config.
- Persist evidence artifacts from default store (`~/.evidra/evidence`) or explicit override path.

**Runtime data flow**
- Pipeline stage submits planned change payload to Skill.
- Skill validates via Evidra and emits machine-readable result.
- Deny fails the stage; allow/observe continues.
- Evidence_id is logged and retained with build artifacts.

### 3) Policy engine (OPA/Rego policy profile)

**Who configures it**
- Policy maintainers and platform security owners.

**Minimal required configuration**
- Policy profile under `policy/bundles/ops-v0.1` (or selected profile reference).
- Deterministic decision contract fields: allow/risk/reason + optional reasons/hits/hints.

**Runtime data flow**
- Skill does not interpret policy logic itself.
- Evidra evaluates policy and returns contract output.
- Skill converts policy output to Skill-level `decision` and `violations`.

### 4) Evidence store

**Who configures it**
- Platform/operations team.

**Minimal required configuration**
- Default path: `~/.evidra/evidence`
- Override with `--evidence-store` (or env `EVIDRA_EVIDENCE_DIR`)
- Retention/backup policy outside skill scope but required operationally.

**Runtime data flow**
- Every validation call returns an `evidence_id`.
- Skill exposes `get_evidence` to fetch stored records.
- Optional chain verification is run periodically or on demand.

## Runtime Flow (Sequence)

### Allow path

```text
Agent -> Skill -> Evidra MCP -> decision=allow + evidence_id
Agent -> Execute planned change
Agent/Platform -> get_evidence(evidence_id) -> audit record retrieved
```

### Deny path

```text
Agent -> Skill -> Evidra MCP -> decision=deny + violations + evidence_id
Agent -> Abort execution (no apply)
Agent/Platform -> get_evidence(evidence_id) -> audit record retrieved
```

## Acceptance Criteria

- **Determinism**: identical input + identical policy/data returns identical decision and violation set.
- **Auditability**: each validation response includes a non-empty, retrievable `evidence_id`.
- **Safety**: `deny` always blocks execution in Skill behavior; `observe` does not block but still records evidence.
- **MCP compatibility**: works with Evidra MCP server `validate` / `get_event` contract without custom protocol changes.
- **Contract stability**: does not require breaking changes to existing Evidra CLI/MCP output formats.
- **Performance target**: local validation should complete within practical CI/agent budgets (target p95 under ~2s for typical payloads).

## Security Considerations

- Apply least privilege to the MCP server runtime and evidence-store filesystem access.
- Treat evidence as sensitive operational metadata; classify, restrict access, and define retention.
- Account for tampering risk at storage layer; use hash-chain verification and controlled write permissions.
- Reduce policy drift risk via policy pinning, review workflows, and test gates (`opa test` + `go test`).
- Preserve provenance: always propagate `evidence_id` into logs, tickets, and CI outputs.

## MVP vs Next

### MVP

- `validate_plan` capability wired to Evidra MCP `validate`.
- `get_evidence` capability wired to Evidra MCP `get_event`.
- Deterministic result contract and strict deny-stop behavior.
- Minimal integration guide for MCP clients and CI usage.

### Next

- Remote evidence forwarder.
- Signed releases and stronger supply-chain controls.
- Policy bundle/profile version pinning strategy.
- Richer context schema (ticket/workflow metadata standards).
- Large payload handling strategy (chunking/reference mode).
- Multi-tenant isolation support.

## Open Questions

- How should context schema versioning be managed across clients and pipeline templates?
- What is the preferred policy distribution model for multiple environments (pin-by-commit, artifact, or package)?
- What retention and rotation model should apply to local evidence stores by default?
- What payload size limits should be enforced for direct MCP tool calls?
- How should environment isolation be represented for multi-cluster / multi-account deployments?
- Should `observe` mode be permitted in production by policy, or only for staged rollout windows?
- What minimum metadata is mandatory in `context` for compliance (ticket_id, actor, source, change window)?


Что конкретно добавить для “skills backend”, не ломая философию

(минимально и очень полезно)

Нормализованный контракт входа
Сейчас у тебя авто-детект файлов (terraform plan json / k8s manifests). Для агентов нужно:

один JSON schema: action_kind, environment, actor, changes[], raw_ref…

чтобы агент мог генерировать структурированный payload без “угадывания форматов”.

Tool: validate уже есть — ок. Добавь explain_rule или policy_info
Чтобы агент мог:

спросить “почему deny”

получить “как исправить” (hint) более прицельно

Режим “proposal”
Чтобы агент мог сделать:

validate на черновик плана

получить hints

переработать план

снова validate

Это почти то же самое, что --observe, но на уровне “помоги улучшить”.