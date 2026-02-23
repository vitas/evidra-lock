# Evidence reference

## Location & layout

- Evidence records live under `~/.evidra/evidence` by default.
- Override via `--evidence-store` (or `--evidence-dir`) on `evidra-mcp`, or via `EVIDRA_EVIDENCE_DIR` (legacy `EVIDRA_EVIDENCE_PATH`) for both binaries.
- The store contains a manifest (`manifest.json`) plus segmented JSONL files under `segments/`.

## What each record holds

- `policy_decision`: `{allow,risk_level,reason}` plus optional `hints`.
- `risk_level` meaning:
  - `low` → decision allowed with no breakglass/risk tags.
  - `medium` → decision allowed but breakglass/exception tags were present.
  - `high` → policy denied or evaluation failed.
- `execution_result`: status/exit codes when a tool was run.
- `params`: contains scenario metadata (`scenario_id`, `scenario_hash`, `action_count`); decision data resides in `policy_decision`.
- Hash chain fields: `previous_hash` links records, and `hash` protects the record contents.

## Inspecting the store

- Look at `manifest.json` for high-level counts and `last_hash`.
- Tail the latest segment: `tail -n 1 ~/.evidra/evidence/segments/evidence-000001.jsonl`.
- Use `go run ./cmd/evidra evidence verify --evidence ~/.evidra/evidence` to validate the chain (advanced).
