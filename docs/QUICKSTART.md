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

- `./bin/evidra validate examples/terraform_plan_pass.json`

## 5) Tests

- `go test ./...`
- `opa test policy/bundles/ops-v0.1/ -v` (policy-level OPA tests)

## Evidence store default

- Default evidence path is always `~/.evidra/evidence`.
- Override for MCP server with `--evidence-store` (or `--evidence-dir`).
- Override for CLI and server with `EVIDRA_EVIDENCE_DIR`.
