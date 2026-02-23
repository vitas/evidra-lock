# Forwarder Cursor v0.1

This document defines a local cursor state used as a progress bookmark for future evidence forwarder/export workflows.

## Purpose

- Track the last acknowledged evidence position in a segmented local evidence store.
- Keep state separate from evidence records and evidence manifest.
- Preserve evidence integrity guarantees by requiring chain validation before cursor updates.

## Location

- `<evidence_root>/forwarder_state.json`

## Schema

```json
{
  "format": "evidra-forwarder-state-v0.1",
  "updated_at": "RFC3339",
  "cursor": {
    "segment": "evidence-000001.jsonl",
    "line": 0
  },
  "last_ack_hash": "<hash>",
  "destination": {
    "type": "none",
    "id": ""
  },
  "notes": ""
}
```

## Rules

- If the state file is missing, cursor state is treated as not set.
- `updated_at` must be updated on every ack operation.
- `last_ack_hash` must match the hash of the record at `cursor.segment` + `cursor.line`.
- Cursor updates must be atomic:
  - write `forwarder_state.json.tmp`
  - rename to `forwarder_state.json`
- Cursor operations must validate evidence chain first.
- Cursor state has no effect on evidence hash-chain integrity and must not modify evidence records.
