# Evidra Regulated Bundle

The Regulated bundle is for controlled operational environments where policy consistency and auditability are primary requirements.

It provides a thin, deterministic validation path over core policy evaluation and is intended for workflows that require explicit decision records before actions proceed.

## What This Bundle Does

- Reads a canonical invocation input.
- Evaluates policy with the shared core runtime.
- Returns a structured decision (`allow`, `risk_level`, `reason`).

Current defaults:

- Policy: `./policy/profiles/ops-v0.1/policy.rego`
- Data: `./policy/profiles/ops-v0.1/data.json`

## Controlled Environment Model

In regulated environments, control is achieved by combining:

- explicit invocation contracts,
- default-deny policy posture,
- immutable evidence storage from the core evidence engine in runtime-integrated deployments.

This separation keeps policy decisions deterministic and auditable.

## Policy Enforcement

Policy is evaluated before execution in the broader Evidra runtime flow.

The regulated bundle itself is intentionally small and focuses on decision correctness:

- no direct execution logic,
- no transport-specific behavior,
- no bundle-to-bundle dependencies.

## Evidence and Audit

Evidence generation is handled by the shared core evidence model/store used by runtime integrations.

For compliance-oriented operations, keep these controls in place:

- append-only evidence storage,
- hash-chain validation,
- exportable audit artifacts,
- stable reason codes for deny/allow outcomes.

## Compliance-Oriented Workflow Example

1. Prepare a controlled invocation JSON.
2. Validate with the regulated bundle:

```bash
evidra regulated validate ./examples/invocations/allowed_terraform_apply_dev.json
```

3. Review output decision and reason.
4. In integrated deployments, verify evidence integrity and export audit artifacts with `evidra-evidence` tooling.

## Operational Guidance

- Keep policy and policy data under change control.
- Treat policy changes as formal change requests.
- Require evidence verification as part of release and incident review.
- Avoid bypass paths outside policy-evaluated execution.
