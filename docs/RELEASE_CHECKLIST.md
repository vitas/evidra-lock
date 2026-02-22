# Release Checklist (v1-slim)

## Build & test

- [ ] `go test ./...`
- [ ] `go build -o ./bin/evidra ./cmd/evidra`

## Smoke validation

- [ ] `./bin/evidra validate bundles/ops/examples/scenario_pass.json`
- [ ] Validate a Terraform plan:
  - `terraform plan -out=plan.tfplan`
  - `terraform show -json plan.tfplan > plan.json`
  - `./bin/evidra validate plan.json`

## Policy snapshot

- [ ] Policy logic: `policy/profiles/ops-v0.1/policy.rego`
- [ ] Policy data: `policy/profiles/ops-v0.1/data.json`
