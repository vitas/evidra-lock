# Quickstart (5-10 Minutes)

## 1) Build

- `go build -o ./bin/evidra ./cmd/evidra`

## 2) Validate a Terraform plan

1. `terraform plan -out=plan.tfplan`
2. `terraform show -json plan.tfplan > plan.json`
3. `./bin/evidra validate plan.json`

## 3) Validate a Kubernetes diff

- Write the diff or rendered manifests to `kube-diff.json`.
- `./bin/evidra validate kube-diff.json`

## 4) Try the bundled scenario

- `./bin/evidra validate bundles/ops/examples/scenario_pass.json`

## 5) Tests

- `go test ./...`
