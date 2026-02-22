# Evidra v1-slim

Evidra v1-slim is a deterministic DevOps validator that reads Terraform plan JSON (or a rendered diff) and returns PASS/FAIL with a policy-backed reason, risk level, and evidence ID.

## 5-minute Terraform plan check

1. `terraform plan -out=plan.tfplan`
2. `terraform show -json plan.tfplan > plan.json`
3. `evidra validate plan.json`

## Sample output

```text
Decision: FAIL
Risk level: high
Evidence: evt-000123
Reason: terraform.plan.destroy-high-risk
```

```text
Decision: PASS
Risk level: low
Evidence: evt-000124
Reason: allowed_write_dev
```

Outputs are deterministic, AI-friendly, and include the evidence ID for audit correlation.

## Policy & evidence

- Policy rules live under `policy/profiles/ops-v0.1/policy/` with structured deny/warn files plus `data.json`.
- Evidence is recorded by default in `./data/evidence` alongside the policy reference used for the decision.

## Try the bundled examples

- `evidra validate examples/terraform_plan_pass.json`
- `evidra validate examples/terraform_mass_delete_fail.json`
- `evidra validate examples/terraform_public_exposure_fail.json`
- Use `--explain` when you want to see the rule IDs, reason, hints, and quick facts for each failing action (e.g., `evidra validate --explain examples/terraform_public_exposure_fail.json`).

## Advanced topics

- `docs/advanced.md` covers MCP, registry/packs, and auxiliary CLI helpers.
- `docs/policy.md` explains the policy layout, rule guidance, and `opa test` workflow.
- `docs/evidence.md` describes the evidence store contents and how to inspect them.
