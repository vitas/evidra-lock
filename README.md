# Evidra v1-slim

Evidra v1-slim is a deterministic validator that reads a Terraform plan or a Kubernetes diff/manifest summary, applies the ops policy profile, and reports PASS/FAIL with a reason, risk level, and evidence ID.

## Install

- `go build ./cmd/evidra`
- `./evidra validate <file>`

## Validate a Terraform plan (5-minute demo)

1. `terraform plan -out=plan.tfplan`
2. `terraform show -json plan.tfplan > plan.json`
3. `./evidra validate plan.json`

## Validate a Kubernetes diff (optional)

- Generate a manifest diff or rendered YAML (for example, with `kubectl diff` or kustomize) and write it to `kube-diff.json`.
- Run `./evidra validate kube-diff.json` to get the same PASS/FAIL output.

## Sample output

```text
Decision: FAIL
Risk level: high
Evidence: evt-1234567890
Reason: terraform.plan.change.requires-approval
```

## Policy profile

- Policy logic: `policy/profiles/ops-v0.1/policy.rego`
- Policy data: `policy/profiles/ops-v0.1/data.json`

## Testing

- `go test ./...`
