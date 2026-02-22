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

- `./bin/evidra validate bundles/ops/examples/scenario_pass.json`
  (bundled scenario files live under `bundles/ops/examples/`)

## Policy profile

- Policy logic: `policy/profiles/ops-v0.1/policy.rego`
- Policy data: `policy/profiles/ops-v0.1/data.json`
