# Demo

## Build

- `go build -o ./bin/evidra ./cmd/evidra`

## Validate a Terraform plan

1. `terraform plan -out=plan.tfplan`
2. `terraform show -json plan.tfplan > plan.json`
3. `./bin/evidra validate plan.json`

## Validate a Kubernetes diff

- Write the diff or rendered YAML to `kube-diff.json` (for example, `kubectl diff > kube-diff.json`).
- `./bin/evidra validate kube-diff.json`

## Validate the sample scenario

- `./bin/evidra validate examples/terraform_plan_pass.json`

## Policy bundle

- OPA bundle: `policy/bundles/ops-v0.1/`
- Policy logic: `policy/bundles/ops-v0.1/evidra/policy/`
- Policy data: `policy/bundles/ops-v0.1/evidra/data/`
