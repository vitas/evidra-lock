# Evidra Ops Bundle

The Ops bundle validates operational scenarios before execution. It is designed for AI-generated or human-authored infrastructure scenarios and produces a clear PASS/FAIL decision plus evidence ID.

## What This Bundle Does

- Accepts a scenario JSON (`scenario_id`, `actor`, `source`, `timestamp`, `actions`).
- Evaluates each action with guardrail policy.
- Returns decision, risk level, reasons, and evidence ID.

## Scenario Format

Top-level required fields:

- `scenario_id`
- `actor.type` (`human|agent|system`)
- `source` (`mcp|cli|ci`)
- `timestamp` (RFC3339)
- `actions` (at least one)

Each action includes:

- `kind` (for example `terraform.plan`)
- `target` (object)
- `intent` (string)
- `payload` (object)
- `risk_tags` (array of strings)

Use built-in explain commands:

```bash
evidra ops explain schema
evidra ops explain kinds
evidra ops explain example
evidra ops explain policies
```

## Supported Action Kinds

Current documented kinds for Ops validation:

- `terraform.plan`
- `kustomize.build`
- `kubectl.apply`
- `kubectl.delete`
- `helm.upgrade`

The evaluator maps `kind` to policy input `tool` + `operation` using `<tool>.<operation>`.

## CLI Usage

```bash
# Validate a scenario file
evidra ops validate ./bundles/ops/examples/scenario_breakglass_audited.json

# Inspect rules in human-readable form
evidra ops explain policies

# Optional: print raw policy
evidra ops explain policies --verbose
```

Output format:

```text
Decision: PASS|FAIL
Risk level: normal|high
Evidence: evt-...
Reason: ...
```

Exit codes:

- `0` when decision is PASS
- non-zero when decision is FAIL or input/runtime error

## Default Guardrails

The default policy is `bundles/ops/policies/policy.rego` and includes these controls:

- Block `k8s.apply` to `kube-system` unless `risk_tags` contains `breakglass`.
- Block `terraform.plan` with `payload.publicly_exposed=true` unless `risk_tags` contains `approved_public`.
- Block `terraform.plan` when `payload.destroy_count > 5`.
- Block actions targeting namespace `prod` unless `risk_tags` contains `change-approved`.
- Block actions with empty or too-short intent (`<10` characters).
- Flag (warn) autonomous execution path (`actor.type=agent` and `source=mcp`) as high risk.

## Realistic Example Scenarios

Included examples:

- `bundles/ops/examples/scenario_s3_public_fail.json`
  - Expected: FAIL (public exposure without approval tag).
- `bundles/ops/examples/scenario_kubesystem_block.json`
  - Expected: FAIL (`kube-system` apply without breakglass).
- `bundles/ops/examples/scenario_breakglass_audited.json`
  - Expected: PASS with high risk and breakglass/audit reasons.

## How To Add Custom Policies

1. Start with `bundles/ops/policies/policy.rego`.
2. Add narrow, deterministic rules using the existing `input` shape from scenario actions.
3. Keep reason codes stable and human-readable.
4. Add or update scenario examples in `bundles/ops/examples/`.
5. Run tests:

```bash
go test ./bundles/ops/...
```

For large policy changes, keep default guards and layer environment-specific rules on top.
