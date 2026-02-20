# Release Checklist (v0.1)

## A) Build and Run

- [ ] `go test ./...`
- [ ] Build binaries:
  - [ ] `go build -o ./bin/evidra-mcp ./cmd/evidra-mcp`
  - [ ] `go build -o ./bin/evidra-policy-sim ./cmd/evidra-policy-sim`
  - [ ] `go build -o ./bin/evidra-evidence ./cmd/evidra-evidence`
- [ ] Start MCP in ops profile:
  - [ ] `EVIDRA_PROFILE=ops EVIDRA_PACKS_DIR=./packs/_core/ops EVIDRA_POLICY_PATH=./policy/kits/ops-v0.1/policy.rego EVIDRA_POLICY_DATA_PATH=./policy/kits/ops-v0.1/data.json EVIDRA_EVIDENCE_PATH=./data/evidence ./bin/evidra-mcp`
- [ ] Confirm startup logs show profile and evidence path.

## B) MCP Functionality

- [ ] `execute` returns structured response with `policy`, `event_id`, and `execution`.
- [ ] Denied `execute` returns `ok=false` with `event_id` and `policy.policy_ref`.
- [ ] `get_event` returns wrapped success payload: `{ "ok": true, "record": {...} }`.
- [ ] Chain invalid condition: `get_event` refuses with `evidence_chain_invalid`.

## C) Evidence Integrity and Forensics

- [ ] Evidence store writes segmented files and manifest.
- [ ] Rotation seals previous segments in manifest.
- [ ] `./bin/evidra-evidence verify --evidence ./data/evidence` passes.
- [ ] Manual tamper check noted: edit a record and verify fails.
- [ ] `./bin/evidra-evidence violations --evidence ./data/evidence --min-risk high` shows expected entries.
- [ ] `./bin/evidra-evidence export --evidence ./data/evidence --out ./audit-pack.tar.gz --policy ./policy/kits/ops-v0.1/policy.rego --data ./policy/kits/ops-v0.1/data.json` creates pack containing:
  - [ ] `evidence/manifest.json`
  - [ ] `evidence/segments/*`
  - [ ] `manifest.json` (audit pack manifest)
  - [ ] policy snapshot files and SHA256 fields (when included)

## D) Docs Sanity

- [ ] `docs/QUICKSTART.md` works as copy-paste flow.
- [ ] `docs/MCP_GUIDE.md` examples match real response fields.
- [ ] `docs/TOOL_PACKS.md` schema matches loader behavior.
- [ ] `docs/OPS_PROFILE.md` matches default profile behavior.

## E) Release Metadata

- [ ] `CHANGELOG.md` updated for release.
- [ ] Version output works:
  - [ ] `./bin/evidra-mcp --version`
  - [ ] `./bin/evidra-policy-sim --version`
  - [ ] `./bin/evidra-evidence --version`
- [ ] Release tag planned: `v0.1.0`.

