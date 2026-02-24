# Release Checklist (v0.1)

## Build & test

- [ ] `go test ./...`
- [ ] `go build -o ./bin/evidra ./cmd/evidra`

## Smoke validation

- [ ] `./bin/evidra validate examples/terraform_plan_pass.json`
- [ ] Validate a plan file:
  - `terraform plan -out=plan.tfplan`
  - `terraform show -json plan.tfplan > plan.json`
  - `./bin/evidra validate plan.json`

## Policy snapshot

- [ ] OPA bundle: `policy/bundles/ops-v0.1/`
- [ ] OPA tests: `opa test policy/bundles/ops-v0.1/ -v`
