# Evidence reference

## Location & layout

- Evidence records live under `./data/evidence` by default (override with `EVIDRA_EVIDENCE_PATH`).
- The store contains a manifest (`manifest.json`) plus segmented JSONL files under `segments/`.

## What each record holds

- `policy_decision`: `{allow,risk_level,reason}` plus optional `hints`.
- `execution_result`: status/exit codes when a tool was run.
- Hash chain fields: `previous_hash` links records, and `hash` protects the record contents.

## Inspecting the store

- Look at `manifest.json` for high-level counts and `last_hash`.
- Tail the latest segment: `tail -n 1 ./data/evidence/segments/evidence-000001.jsonl`.
- Use `go run ./cmd/evidra evidence verify --evidence ./data/evidence` to validate the chain (advanced).
