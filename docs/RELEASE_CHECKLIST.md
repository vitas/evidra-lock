# Release Checklist (v1-slim)

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

- [ ] Policy logic: `policy/profiles/ops-v0.1/policy.rego`
- [ ] Policy data: `policy/profiles/ops-v0.1/data.json`
